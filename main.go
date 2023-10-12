package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	concurrency int
	verbose     bool
	domains     chan check
)

type check struct {
	Domain     string
	Nameserver string
}

func queryNSEC(domain string, server string, visited *sync.Map) {
	if _, found := visited.LoadOrStore(domain, true); found {
		if verbose {
			fmt.Println("Loop detected, stopping recursion.")
		}
		return
	}

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeNSEC)

	r, _, err := c.Exchange(m, server)
	if err != nil {
		if verbose {
			fmt.Println(err)
		}

		return
	}

	for _, ans := range r.Answer {
		if nsec, ok := ans.(*dns.NSEC); ok {
			if verbose {
				fmt.Println("Next Domain:", nsec.NextDomain)
			} else {
				fmt.Println(nsec.NextDomain)
			}
			queryNSEC(nsec.NextDomain, server, visited)
		}
	}
}

func worker(visited *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()
	for domain := range domains {
		queryNSEC(domain.Domain, domain.Nameserver, visited)
	}
}

func main() {
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")
	flag.BoolVar(&verbose, "v", false, "be more verbose")
	flag.Parse()

	dnsServers := []string{
		"1.1.1.1:53",
		"1.0.0.1:53",
		"8.8.8.8:53",
		"8.8.4.4:53",
		"9.9.9.9:53",
	}
	rand.Seed(time.Now().UnixNano())

	visited := &sync.Map{}
	domains = make(chan check, concurrency)

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(visited, &wg)
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
