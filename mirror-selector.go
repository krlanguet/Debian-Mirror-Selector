package main

import (
    // Argument Parsing
    "github.com/docopt/docopt-go"

    // Logging
    "os"
    "io"
    "io/ioutil"
    "log"
    "github.com/davecgh/go-spew/spew"

    // File IO
    "bufio"

     // Requesting Webpages
    "net/http"
)

type Logger struct {
    *log.Logger
    out io.Writer
}

func (l *Logger) Dump(a ...interface{}) {
    spew.Fdump(l.out, a...)
}

func NewLogger(logOn bool) Logger {
    var out io.Writer
    if logOn {
        out = os.Stdout
    } else {
        out = ioutil.Discard
    }
    return Logger{
        Logger: log.New(out, "", log.LstdFlags),
        out: out,
    }
}

var logger = NewLogger(true)

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
    logger.Dump(arguments)

    //  Main will spawn the following coroutines:
    //      * File Reader           - Asynchronously parses file
    //      * Scoring Dispatcher    - Receives sites from Reader and spawns scorers
    //          * Scorers
    //  And the blocking call:
    //      * Results Accumulator   - Awaits completion of Scorers created by Dispatcher
    //
    //  All of which will communicate over the following channels:

    siteBufferSize := 32
    parsedSites := make(chan *site, siteBufferSize)
    // Buffered site* channel to asynchronously parse file

    //scorerCreated := make(chan bool)
    // Blocking bool channek so the Accumulator always counts the creation of a Scorer before 
    //  receiving its score.

    noMoreScorers := make(chan bool, 1)
    // Bool channel to inform the Accumulator that it can start counting down to completion.

    //scoreBufferSize := 32
    //scores := make(chan *site, scoreBufferSize)
    // Buffered site* channel so finished scorers will exit without waiting on the Accumulator,
    //  which would otherwise waste memory.
    // NOTE: This depends on the relationship between Scoring Dispatcher limiting and scores
    //  buffer size

    go mirrorListReader("", parsedSites)
    
    go scoringDispatcher(parsedSites, noMoreScorers)
    
    resultsAccumulator(noMoreScorers)
}

type site struct {
    dumbyVar string
}

//  The File Reader will:
//      Read INFILE or Request 'http://www.debian.org/mirror/list-full'
//      Scan for sites, sending into parsedSites
//      When all scanned, close parsedSites and exit
func mirrorListReader(INFILE string, parsedSites chan *site) {
    defer close(parsedSites)
    var reader *bufio.Reader
    
    if INFILE != "" {
        file, err := os.Open(INFILE)
        if err != nil {
            logger.Fatalln(err)
        }
        reader = bufio.NewReader(file)
        defer file.Close()
    } else {
        resp, err := http.Get("https://www.debian.org/mirror/list-full")
        if err != nil {
            logger.Fatalln(err)
        }
        defer resp.Body.Close()
        reader = bufio.NewReader(resp.Body)
    }

    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        s := &site{dumbyVar: scanner.Text()}
        parsedSites <- s
    }
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
func scoringDispatcher(parsedSites chan *site, noMoreScorers chan bool) {
    for s := range parsedSites {
        logger.Println(s.dumbyVar)   
    }
    noMoreScorers <- true
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
func resultsAccumulator(noMoreScorers chan bool) {
    <- noMoreScorers
}
