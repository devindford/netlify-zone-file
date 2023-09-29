package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

const urlPrefix string = "https://api.netlify.com/api/v1/"

type DnsZone struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type DnsRecord struct {
	Id        string  `json:"id"`
	DnsZoneId string  `json:"dns_zone_id"`
	Hostname  string  `json:"hostname"`
	Type      string  `json:"type"`
	Ttl       int     `json:"ttl"`
	Priority  int     `json:"priority"`
	Weight    *int    `json:"weight,omitempty"`
	Port      *int    `json:"port,omitempty"`
	Flag      *string `json:"flag,omitempty"`
	Tag       *string `json:"tag,omitempty"`
	Managed   bool    `json:"managed"`
	Value     string  `json:"value"`
}

type NetlifyDnsClient struct {
	client *http.Client
	token  string
}

type Redirect struct {
	From   string `toml:"from"`
	To     string `toml:"to"`
	Status int    `toml:"status"`
	Force  bool   `toml:"force"`
}

type NetlifyToml struct {
	Redirects []Redirect `toml:"redirects"`
}

func readNetlifyToml(filePath string) (NetlifyToml, error) {
	var config NetlifyToml
	content, err := os.ReadFile(filePath)
	if err != nil {
		return config, err
	}

	err = toml.Unmarshal(content, &config)
	return config, err
}

func NewNetlifyDnsClient(token string) NetlifyDnsClient {
	client := &http.Client{}

	return NetlifyDnsClient{client: client, token: token}
}

func (n *NetlifyDnsClient) addAuthHeader(req *http.Request) {
	req.Header.Add("Authorization", "Bearer "+n.token)
}

func (n *NetlifyDnsClient) getReqByteSlice(endpoint string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlPrefix+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating get request: %w", err)
	}
	n.addAuthHeader(req)

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing get request: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading get request body: %w", err)
	}

	return body, nil
}

func (n *NetlifyDnsClient) GetAllDnsZones() ([]DnsZone, error) {
	body, err := n.getReqByteSlice("dns_zones")
	if err != nil {
		return nil, err
	}

	var dnsZones []DnsZone
	err = json.Unmarshal(body, &dnsZones)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling get request body: %w", err)
	}

	return dnsZones, nil
}

func (n *NetlifyDnsClient) GetAllDnsRecords(zoneId string) ([]DnsRecord, error) {
	body, err := n.getReqByteSlice("dns_zones/" + zoneId + "/dns_records")
	if err != nil {
		return nil, err
	}

	var records []DnsRecord
	err = json.Unmarshal(body, &records)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling get request body: %w", err)
	}

	return records, nil
}

func GenerateZoneFile(zone DnsZone, records []DnsRecord, redirects []Redirect) (string, error) {
	var zoneFile strings.Builder

	zoneFile.WriteString(fmt.Sprintf("$ORIGIN %s\n", zone.Name+"."))

	// Create a map to track processed record names
	processedNames := make(map[string]bool)

	for _, record := range records {
		name := record.Hostname + "."

		// Check if this record name has been processed
		if _, exists := processedNames[name]; exists {
			fmt.Printf("Ignoring duplicate record name: %s\n", record.Hostname)
			continue
		}

		// Now mark this record name as processed
		processedNames[name] = true

		var value string
		if record.Type == "CNAME" || record.Type == "NETLIFYv6" || record.Type == "NETLIFY" {
			value = record.Value + "."
		} else {
			value = record.Value
		}

		var priority = ""
		if record.Priority != 0 {
			priority = fmt.Sprintf("\t%d", record.Priority)
		}

		for _, redirect := range redirects {
			if matchRedirectRule(record.Hostname, redirect.From) {
				value = extractDestination(redirect.To)
				fmt.Printf("redirect: %s", redirect.To)
				break
			}
		}

		zoneFile.WriteString(
			fmt.Sprintf(
				"%s\tIN\t%d\t%s%s\t%s\n",
				name,
				record.Ttl,
				typeWithReplacement(record.Type),
				priority,
				value,
			),
		)
	}

	return zoneFile.String(), nil
}

func typeWithReplacement(recordType string) string {
	if recordType == "NETLIFY" || recordType == "NETLIFYv6" {
		return "CNAME"
	}
	return recordType
}

// Checks if the domain name matches the "from" part of the redirect rule
func matchRedirectRule(domain, fromRule string) bool {
	parsedURL, err := url.Parse(fromRule)
	if err != nil {
		// Handle error or log it
		return false
	}
	return domain == parsedURL.Host
}

// Extracts the destination URL from the "to" part of the redirect rule
// and removes any :splat from the URL since it will be handled at the app level
func extractDestination(toRule string) string {
	// Remove :splat or any other placeholder from the URL
	cleanedURL := strings.ReplaceAll(toRule, ":splat", "")

	// Optionally, you might want to clean up URLs that end with a slash as a result of removing :splat
	cleanedURL = strings.TrimRight(cleanedURL, "/")

	return cleanedURL
}

func main() {
	token := os.Getenv("NETLIFY_TOKEN")

	if token == "" {
		log.Fatalln("NETLIFY_TOKEN was not set.")
	}

	client := NewNetlifyDnsClient(token)

	zones, err := client.GetAllDnsZones()
	if err != nil {
		log.Fatalln(err)
	}
	for _, zone := range zones {
		records, err := client.GetAllDnsRecords(zone.Id)
		if err != nil {
			log.Fatalln(err)
		}

		tomlConfig, err := readNetlifyToml("netlify.toml")
		if err != nil {
			log.Fatalf("Failed to read netlify.toml: %v", err)
		}

		zoneContents, err := GenerateZoneFile(zone, records, tomlConfig.Redirects)
		if err != nil {
			log.Fatalln(err)
		}

		fileName := zone.Id + ".zone"

		err = os.WriteFile(fileName, []byte(zoneContents), 0644)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(fileName)
	}
}
