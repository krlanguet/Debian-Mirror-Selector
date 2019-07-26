package main

import (
	// Argument Parsing
	"github.com/docopt/docopt-go"

	// Logging
	"github.com/krlanguet/debian-mirror-selector/logger"
	"time"

	// File IO
	"os"

	// Mirror List Parsing
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

var usage = `Name:
Debian-Mirror-Selector - Creates a sources.list file with the fastest Debian package mirrors that fit specified criteria.

Description:
    Filters mirrors (by default: parsed from https://www.debian.org/mirror/list-full) for those
     which serve the given version (default: stable), support the given architecture (default: that
     of current machine, as reported by dpkg), and respond to the given protocols (default: HTTPS).
     These mirrors are sorted by a netelect-inspired ping implementation, and used to construct
     the output file (default: ./sources.list).

Example:
    mirror-selector --release unstable --protocols https,ftp

Usage:
    mirror-selector [-ns] [-p <P1,P2,...>] [-a <ARCH>] [-r <RELEASE>] [-o <OUTFILE>] [<INFILE>]
    mirror-selector (-h | --help)
    mirror-selector (-v | --version)

Options:
   INFILE                    File to read mirrors from. Must have same formatting as
                               https://www.debian.org/mirror/list-full.
   -o --out-file OUTFILE     File to output to [default: ./sources.list]. Writes in sources.list
                               format regardless.
   -n --nonfree              Output file will also include non-free sections.
   -s --source-packages      Output file will include deb-src lines for use with apt-get source
                               to obtain Debian source packages.
   -p --protocols P1,P2,...  Protocols which mirrors must serve on [default: https].
  
   -a --architecture ARCH    Which architecture to look for. Accepts any of:
                               all, amd64, arm64, armel, armhf, hurd-i386, i386, ia64,
                               kfreebsd-amd64, kfreebsd-i386, mips, mips64el, mipsel, powerpc,
                               ppc64el, s390, s390x, source, or sparc. Defaults to consulting
                               dpkg for current machine architecture.
                                                        
   -r --release RELEASE      Which Debian release to look for [default: stable]. Accepts
                               targets (stable, testing, unstable, or experimental) or 
                               code names (wheezy, jessie, stretch, ... etc.).
   -h --help                 Prints this help text.
   -v --version              Prints the version information.
`

// This program uses the following architecture:
//  - Main parses file into sites
//  - Main spawns Scoring Dispatcher
//      - Dispatcher filters sites and spawns Scorers
//          - Scorers connect and profile each site
//  - Main calls Accumulator
//      - Acc. counts created scorers
//      - Acc. collects completed work from dispatched Scorers
//  - Main writes the output file
//
//  Routines communicate over the following channels:

var scorerCreated = make(chan bool)

// Blocking bool channel so the Accumulator always counts the creation of a Scorer before
//  receiving its score.

var noMoreScorers = make(chan bool, 1)

// Bool channel to inform the Accumulator that it can start counting down to completion.

var scoreBufferSize = 32
var scores = make(chan *site, scoreBufferSize)

// Buffered site* channel so finished scorers will typically exit without waiting on the
//  Accumulator, which would otherwise waste memory.
// NOTE: This depends on the relationship between Scoring Dispatcher limiting and scores
//  buffer size

var log = logger.New(true)

func main() {
	start := time.Now()
	arguments, _ := docopt.ParseDoc(usage)
	cliArgsParsed := time.Now()

	// Load document for parsing
	var doc *html.Node
	var err error
	if arguments["<INFILE>"] == nil {
		doc, err = htmlquery.LoadURL("https://www.debian.org/mirror/list-full")
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		file, err := os.Open(arguments["<INFILE>"].(string))
		if err != nil {
			log.Fatalln(err)
		}

		doc, err = htmlquery.Parse(file)
		if err != nil {
			log.Fatalln(err)
		}
		file.Close()
	}

	documentLoaded := time.Now()

	// Parse HTML tree for markers
	contentDiv := htmlquery.FindOne(doc, "/html/body/div[@id='content']")
	countryDivs := htmlquery.Find(contentDiv, "/h3")
	siteDivs := htmlquery.Find(contentDiv, "/text()[normalize-space(.)='Site:']")
	packageURLDivs := htmlquery.Find(contentDiv, "/text()[starts-with(normalize-space(.), 'Packages over ')]")
	archDivs := htmlquery.Find(contentDiv, "/text()[starts-with(normalize-space(.), 'Includes architectures: ')]")
	typeDivs := htmlquery.Find(contentDiv, "/text()[starts-with(normalize-space(.), 'Type: ')]")
	breakDivs := htmlquery.Find(contentDiv, "/br")

	log.Println("Found", len(countryDivs), "countries.")
	log.Println("Found", len(siteDivs), "sites.")
	log.Println("Found", len(packageURLDivs), "package URLs.")

	// Storage for Sites
	sites := make([]*site, 0)

	// Loop through sibling nodes in document, searching for prefixs
	// Current state through loop, starts with none found
	countryIndex := -1
	siteIndex := -1
	packageURLIndex := -1
	archIndex := -1
	typeIndex := -1
	breakIndex := -1
	node := countryDivs[0].PrevSibling
	var s site

	for {
		node = node.NextSibling
		if node == nil {
			// We've reached end of document when there are no more siblings
			sites = append(sites, &s)
			break
		} else if breakIndex+1 < len(breakDivs) && node == breakDivs[breakIndex+1] {
			breakIndex++
			continue
		} else if countryIndex+1 < len(countryDivs) && node == countryDivs[countryIndex+1] {
			// Country prefix
			countryIndex++
		} else if siteIndex+1 < len(siteDivs) && node == siteDivs[siteIndex+1] {
			// Site prefix
			// Save old site
			if siteIndex != -1 {
				sites = append(sites, &s)
			}
			// Make new site
			siteIndex++
			s = site{PackProtocols: make(map[string]*url.URL)}
			// Record site url
			node = node.NextSibling
			if node == nil || htmlquery.FindOne(node, "self::tt") == nil {
				log.Fatalln("Parsing site URL failed")
			}
			s.Hosts = strings.Split(htmlquery.InnerText(node), ",")
		} else if packageURLIndex+1 < len(packageURLDivs) && node == packageURLDivs[packageURLIndex+1] {
			// Package URL prefix
			packageURLIndex++
			// Read protocol
			protocol := strings.TrimPrefix(htmlquery.InnerText(node), "Packages over ")
			protocol = strings.TrimSuffix(protocol, ": ")
			// Read URL
			node = node.NextSibling
			if node == nil || htmlquery.FindOne(node, "self::tt") == nil {
				log.Fatalln("Parsing package URL failed")
			}
			var URL *url.URL
			switch protocol {
			case "HTTP":
				URL, err = url.Parse(htmlquery.SelectAttr(node.FirstChild, "href"))
				if err != nil {
					log.Fatalln(err)
				}
				URL.Scheme = "http"
			case "rsync":
				// Resolve relative rsync URL
				URL = &url.URL{Scheme: "rsync", Host: s.Hosts[0]}
				URL.Path = strings.TrimSpace(htmlquery.InnerText(node))
			}
			s.PackProtocols[protocol] = URL
		} else if typeIndex+1 < len(typeDivs) && node == typeDivs[typeIndex+1] {
			// Type prefix
			typeIndex++
			s.SiteType = strings.TrimPrefix(strings.TrimSpace(htmlquery.InnerText(node)), "Type: ")
		} else if archIndex+1 < len(archDivs) && node == archDivs[archIndex+1] {
			archIndex++
			archListString := htmlquery.InnerText(node)
			archListString = strings.TrimSpace(archListString)
			archListString = strings.TrimPrefix(archListString, "Includes architectures: ")
			s.Architectures = strings.Split(archListString, " ")
		} else {
			//log.Println("Ignoring token:", htmlquery.OutputHTML(node, true))
		}
	}

	docParsed := time.Now()

	/*
		log.Println(len(sites))
		for _, s := range sites[:10] {
			log.Dump(s)
		}
	*/

	go scoringDispatcher(sites)

	results := resultsAccumulator()

	scoringDone := time.Now()

	log.Println(results)
	log.Println("Parsing CLI Arguments took", cliArgsParsed.Sub(start))
	log.Println("Loading document took", documentLoaded.Sub(cliArgsParsed))
	log.Println("Parsing document took", docParsed.Sub(documentLoaded))
	log.Println("Scoring took", scoringDone.Sub(docParsed))
}

type site struct {
	Hosts         []string
	SiteType      string
	Architectures []string
	PackProtocols map[string]*url.URL
	//UpdateFrequency string
	Score int
}

//  The Scoring Dispatcher will:
//      Iterate over sites:
//          If site matches all filtering criteria:
//              Send into scorerCreated
//              Spawn a Scorer coroutine
//      When all sites have been found:
//          Send true into noMoreScorers
//          Exit
func scoringDispatcher(sites []*site) {
	for _, s := range sites {
		if true {
			scorerCreated <- true
			go score(s)
		}
	}
	noMoreScorers <- true
}

//  Each Scorer will:
//      Try connecting over desired protocols
//      If connection fails:
//          Send worst score into scores and exit
//      Run ping/traceroute algorithm
//      Whether succeeds or times out, send into scores and exit
func score(s *site) {
	s.Score = 0
	scores <- s
}

//  The Results Accumulator will:
//      Infinitely select over:
//          scorerCreated:
//              Increment count of active scorers
//          noMoreScorers:
//              set done variable to true
//          scores:
//              Push site on a best-score heap
//              Decrement active scorers count
//              If done and count is zero:
//                  Break out of infinite select loop
//      Pop sites off of heap.
//      Format sites and write to OUTFILE.
//      Exit
func resultsAccumulator() []int {
	results := make([]int, 0)
	done := false
	scorers := 0
	for {
		select {
		case <-scorerCreated:
			scorers++
		case <-noMoreScorers:
			done = true
			if scorers == 0 {
				return results
			}
		case s := <-scores:
			//log.Println("Score received:", s.Score)
			results = append(results, s.Score)
			//Heap
			scorers--
			if done && scorers == 0 {
				return results
			}
		}
	}

}
