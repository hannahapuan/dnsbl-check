package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/Ajnasz/dnsbl-check/dnsblprovider"
	"github.com/Ajnasz/dnsbl-check/providerlist"
)

// LookupResult stores the query result with reason
type LookupResult struct {
	IsBlacklisted bool
	Address       string
	Reason        string
	Provider      dnsblprovider.DNSBLProvider
	Error         error
}

func Lookup(address string, provider dnsblprovider.DNSBLProvider) LookupResult {
	isListed, err := provider.IsBlacklisted(address)
	if err != nil {
		return LookupResult{
			Provider: provider,
			Address:  address,
			Error:    err,
		}
	}

	if isListed {
		desc, err := provider.GetReason(address)

		return LookupResult{
			Error:         err,
			Address:       address,
			IsBlacklisted: true,
			Provider:      provider,
			Reason:        desc,
		}
	}

	return LookupResult{
		Address:       address,
		IsBlacklisted: false,
		Provider:      provider,
	}
}

func getBlacklists(addresses []string, providers []dnsblprovider.DNSBLProvider) chan LookupResult {
	var wg sync.WaitGroup
	results := make(chan LookupResult)
	for _, address := range addresses {
		for _, provider := range providers {
			wg.Add(1)
			go func(address string, provider dnsblprovider.DNSBLProvider) {
				defer wg.Done()
				results <- lookup(address, provider)
			}(address, provider)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func processLookupResult(result LookupResult) {
	if result.Error != nil {
		fmt.Println(fmt.Sprintf("ERR\t%s\t%s\t%s", result.Address, result.Provider.GetName(), result.Error))
		return
	}
	if result.IsBlacklisted {
		var reason string

		if result.Reason == "" {
			reason = "unkown reason"
		} else {
			reason = result.Reason
		}

		fmt.Println(fmt.Sprintf("FAIL\t%s\t%s\t%s", result.Address, result.Provider.GetName(), reason))
	} else {
		fmt.Println(fmt.Sprintf("OK\t%s\t%s", result.Address, result.Provider.GetName()))
	}
}

func main() {
	var domainsFile = flag.String("p", "", "path to file which stores list of dnsbl checks, empty or - for stdin")
	var addressesParam = flag.String("i", "", "IP Address to check, separate by comma for a list")

	flag.Parse()
	list, err := providerlist.GetProvidersChan(*domainsFile)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading domains")
		os.Exit(1)
	}

	var providers []dnsblprovider.DNSBLProvider

	for item := range list {
		provider := dnsblprovider.GeneralProvider{
			URL: item,
		}

		providers = append(providers, provider)
	}

	addresses := providerlist.GetAddresses(*addressesParam)
	for result := range getBlacklists(addresses, providers) {
		processLookupResult(result)
	}
}
