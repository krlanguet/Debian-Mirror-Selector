package main

import (
    // Argument Parsing
    "github.com/docopt/docopt-go"

    // Logging
    "github.com/davecgh/go-spew/spew"
)

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
    spew.Dump(arguments)
}
