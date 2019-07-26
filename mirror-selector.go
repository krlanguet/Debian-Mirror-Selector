package main

import (
	// Argument Parsing
	"github.com/docopt/docopt-go"

	// Logging
	"github.com/krlanguet/debian-mirror-selector/logger"

	"bufio"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"os"
)

var log = logger.New(true)

func main() {
	usage := `Name:
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
	arguments, _ := docopt.ParseDoc(usage)
	//log.Dump(arguments)

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

	//scorerCreated := make(chan bool)
	// Blocking bool channel so the Accumulator always counts the creation of a Scorer before
	//  receiving its score.

	//noMoreScorers := make(chan bool, 1)
	// Bool channel to inform the Accumulator that it can start counting down to completion.

	//scoreBufferSize := 32
	//scores := make(chan *site, scoreBufferSize)
	// Buffered site* channel so finished scorers will exit without waiting on the Accumulator,
	//  which would otherwise waste memory.
	// NOTE: This depends on the relationship between Scoring Dispatcher limiting and scores
	//  buffer size

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

		doc, err = htmlquery.Parse(bufio.NewReader(file))
		if err != nil {
			log.Fatalln(err)
		}
		file.Close()
	}

	// Parse HTML tree for markers
	contentDiv := htmlquery.FindOne(doc, "/html/body/div[@id='content']")
	countryDivs := htmlquery.Find(contentDiv, "/h3")
	siteDivs := htmlquery.Find(contentDiv, "/text()[normalize-space(.)='Site:']")

	log.Println("Found", len(countryDivs), "countries.")
	log.Println("Found", len(siteDivs), "sites.")

	// Storage for Sites
	sites := make([]site, 0)

	// Loop through sibling nodes in document
	// Current state through loop, starts with none found
	countryIndex := -1
	siteIndex := -1
	node := countryDivs[0].PrevSibling
	var s site

	for true {
		node = node.NextSibling
		if node == nil {
			// We've reached end of document when there are no more siblings
			sites = append(sites, s)
			break
		} else if countryIndex+1 < len(countryDivs) && node == countryDivs[countryIndex+1] {
			// Mark that we've entered a new country
			countryIndex++
		} else if siteIndex+1 < len(siteDivs) && node == siteDivs[siteIndex+1] {
			// Mark that we've entered a new site, saving the old to the slice
			if siteIndex != -1 {
				sites = append(sites, s)
			}
			siteIndex++
			s = site{Strings: make([]string, 0)}
		} else {
			// Parse this node for site content
			s.Strings = append(s.Strings, htmlquery.OutputHTML(node, true))
		}
	}

	/*
		log.Println(len(sites))
		for _, s := range sites[:5] {
	    	    log.Println(s.Strings)
		}
	*/

	//log.Println(htmlquery.OutputHTML(mirrorListDiv, true))

	//go scoringDispatcher(parsedSites, noMoreScorers)

	//resultsAccumulator(noMoreScorers)
}

type site struct {
	Strings         []string
	SiteUrl         string
	SiteType        string
	Architectures   []string
	Protocols       []string
	UpdateFrequency string
}

//  The Scoring Dispatcher will:
//      Iterate over parsedSites:
//          If site matches all filtering criteria:
//              Send into scorerCreated
//              Spawn a Scorer coroutine
//      When all sites have been found:
//          Send true into noMoreScorers
//          Exit

//  Each Scorer will:
//      Try connecting over desired protocols
//      If connection fails:
//          Send worst score into scores and exit
//      Run ping/traceroute algorithm
//      Whether succeeds or times out, send into scores and exit
//func scoringDispatcher(parsedSites chan *site, noMoreScorers chan bool) {
//    for s := range parsedSites {
//        log.Println(s.dumbyVar)
//    }
//    noMoreScorers <- true
//}

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
//func resultsAccumulator(noMoreScorers chan bool) {
//    <- noMoreScorers
//}
