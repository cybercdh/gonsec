package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	concurrency     int
	verbose         bool
	maxRetries      int
	onlineresolvers bool
	domains         chan check
)

type check struct {
	Domain     string
	Nameserver string
}

type DNSServer struct {
	IP          string  `json:"ip"`
	Name        string  `json:"name"`
	Reliability float64 `json:"reliability"`
}

func queryNSEC(domain string, servers []string, visited *sync.Map, retryCount int) {
	// Check if the domain has been visited before
	if _, found := visited.Load(domain); found {
		if verbose {
			fmt.Println("Loop detected, stopping recursion.")
		}
		return
	}

	// Select a random DNS server
	server := servers[rand.Intn(len(servers))]

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeNSEC)

	r, _, err := c.Exchange(m, server)
	if err != nil {
		if verbose {
			fmt.Println("Error querying DNS:", err)
		}

		// Retry with a different DNS server if retry count is not zero
		if retryCount > 0 {
			if verbose {
				fmt.Println("Retrying with a different DNS server...")
			}

			queryNSEC(domain, servers, visited, retryCount-1)
		} else {
			if verbose {
				fmt.Println("Max retries reached. Skipping", domain)
			}
		}
		return
	}

	// Mark the domain as visited only after a successful query
	visited.Store(domain, true)

	for _, ans := range r.Answer {
		if nsec, ok := ans.(*dns.NSEC); ok {
			if verbose {
				fmt.Println("Next Domain:", nsec.NextDomain)
			} else {
				fmt.Println(nsec.NextDomain)
			}

			queryNSEC(nsec.NextDomain, servers, visited, retryCount)
		}
	}
}

func worker(visited *sync.Map, wg *sync.WaitGroup, servers []string, maxRetries int) {
	defer wg.Done()
	for domain := range domains {
		queryNSEC(domain.Domain, servers, visited, maxRetries) // Use domain.Domain here
	}
}

func getDNSServers(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	// Assuming the CSV has a header row
	_, err = reader.Read()
	if err != nil {
		return nil, err
	}

	var servers []string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Assuming IP is in the first column and reliability is in the tenth column
		ip := record[0]
		reliability := record[9]

		// Assuming reliability "1.00" means fully reliable
		if reliability == "1.00" {
			servers = append(servers, ip+":53")
		}
	}

	return servers, nil
}

func main() {
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")
	flag.IntVar(&maxRetries, "r", 3, "set the number of retries")
	flag.BoolVar(&verbose, "v", false, "be more verbose")
	flag.BoolVar(&onlineresolvers, "o", false, "use online list of DNS resolvers")
	flag.Parse()

	var dnsServers []string
	var err error

	if onlineresolvers {
		dnsServersURL := "https://public-dns.info/nameservers.csv"
		dnsServers, err = getDNSServers(dnsServersURL)
		if err != nil {
			log.Fatalf("Error fetching DNS servers: %v", err)
		}

	} else {
		// if the user doesn't want to use the big online list, then throw a few regular ones in the mix
		dnsServers = []string{
			"1.1.1.1:53",
			"1.0.0.1:53",
			"8.8.8.8:53",
			"8.8.4.4:53",
			"9.9.9.9:53",
		}
	}

	rand.Seed(time.Now().UnixNano())

	visited := &sync.Map{}
	domains = make(chan check, concurrency)

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(visited, &wg, dnsServers, maxRetries)
	}

	if _, err := GetUserInput(dnsServers); err != nil {
		log.Println(err)
	}

	close(domains)
	wg.Wait()
}

func GetUserInput(dnsServers []string) (bool, error) {
	seen := make(map[string]bool)

	var inputDomains io.Reader
	inputDomains = os.Stdin

	argDomain := flag.Arg(0)
	if argDomain != "" {
		inputDomains = strings.NewReader(argDomain)
	}

	sc := bufio.NewScanner(inputDomains)

	for sc.Scan() {
		domain := strings.ToLower(sc.Text())
		if _, ok := seen[domain]; ok {
			continue
		}

		seen[domain] = true
		server := dnsServers[rand.Intn(len(dnsServers))]
		domains <- check{domain, server}
	}

	if err := sc.Err(); err != nil {
		return false, err
	}

	return true, nil
}
