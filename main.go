package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/junglehornet/goscan"
	fastping "github.com/tatsushid/go-fastping"
)

// global flag variables
var verbose bool
var outFile string
var threadCount int

// global statistics variables
var startTime time.Time
var scanned int
var total int
var done bool

// channel where all IPs that respond are put (without message, xxx.xxx.xxx.xxx format)
var resChan chan string

func printErrString(format string, vars ...any) {
	fmt.Printf(format, vars...)
	os.Exit(1)
}

func printErr(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func main() {
	if os.Geteuid() != 0 {
		printErrString("Error: %s must be run as root!\n", os.Args[0])
	}
	parseFlags()

	scanned = 0
	done = false

	fmt.Println("Generating IP List...")

	ips := parseRange()

	fmt.Printf("IP List generation complete, scanning %d IPs.\n", cap(ips))
	startTime = time.Now()

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT)

	go func() {
		<-sigs
		fmt.Println("interrupt recieved, stopping scanning early.")
		done = true
		time.AfterFunc(5*time.Second, func() { printErrString("error: execution did not stop withing 5 seconds of SIGINT recieved\n") })
	}()

	resChan = make(chan string, cap(ips))
	idles := make(chan string, cap(ips))
	responses := make(chan string, cap(ips))
	fmt.Println("Starting goroutines...")
	for i := 0; i < threadCount; i++ {
		if done {
			break
		}
		if len(ips) == 0 {
			break
		}
		go runPings(ips, responses, idles, verbose)
		msg := fmt.Sprintf("%d/%d", i+1, threadCount)
		if i != threadCount {
			msg += strings.Repeat("\b", len(msg))
		}
		fmt.Print(msg)
	}
	fmt.Println("\nAll goroutines running.\n")

	if !verbose {
		time.AfterFunc(5*time.Second, printUpdate)
	}

	responsesList := []string{}
	for i := 0; i < cap(ips); i++ {
		if done {
			break
		}
		if verbose {
			if len(responses) > 0 {
				res := <-responses
				responsesList = append(responsesList, res)
				fmt.Print(res)
			}
			fmt.Print(<-idles)
		} else {
			<-idles
		}
	}
	done = true
	totalTime := time.Since(startTime)
	for len(responses) > 0 {
		responsesList = append(responsesList, <-responses)
	}
	fmt.Printf("\n|======== Responses: (%d) ========\n", len(responsesList))
	for i, response := range responsesList {
		if i < 25 {
			fmt.Printf("| %s", response)
		} else if i == 26 {
			fmt.Printf("| ... %d entries have been truncated.\n", len(responsesList)-25)
		}
	}
	fmt.Printf("\n|======== Statistics: ========\n")
	fmt.Printf("|Total requests sent: %d\n", scanned)
	fmt.Printf("|Total responses recieved: %d/%d (%.2f%%)\n", len(responsesList), scanned, float64(len(responsesList))/float64(scanned)*100)
	fmt.Printf("|Scanning took %vs\n", totalTime.Seconds())
	if outFile != "" {
		fmt.Printf("\nSaving IPs to \"%s\"...\n", outFile)
		saveToFile()
	}
}

func runPings(ips chan string, responses chan string, idles chan string, verbose bool) {
	for !done {
		addrString := <-ips

		p := fastping.NewPinger()
		ra, err := net.ResolveIPAddr("ip4:icmp", addrString)
		if err != nil {
			printErr(err)
		}
		p.AddIPAddr(ra)
		p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
			message := fmt.Sprintf("[RESPONSE RECIEVED] IP Addr: %s, RTT: %v\n", addr.String(), rtt)
			responses <- message
			resChan <- addrString
		}
		p.OnIdle = func() {
			message := fmt.Sprintf("IP Scanned: %s\n", addrString)
			idles <- message
			scanned++
		}
		err = p.Run()
		if err != nil {
			printErr(err)
		}
	}
	// put any cleanup code here
}

func parseRange() chan string {
	if !flag.Parsed() {
		flag.Parse()
	}
	rangeStr := flag.Arg(0)
	ranges := strings.Split(rangeStr, ".")
	if len(ranges) != 4 {
		printErrString("Error: Invalid IP range \"%s\"\n", rangeStr)
	}

	rangeList := make([]int, 8)

	for i, each := range ranges {
		i *= 2
		twoNums := strings.Split(each, "-")
		var err error
		rangeList[i], err = strconv.Atoi(twoNums[0])
		if err != nil {
			printErr(err)
		}
		if len(twoNums) == 1 {
			rangeList[i+1], err = strconv.Atoi(twoNums[0])
		} else {
			rangeList[i+1], err = strconv.Atoi(twoNums[1])
		}
		if err != nil {
			printErr(err)
		}
	}

	a1 := rangeList[0]
	a2 := rangeList[1]
	aRange := a2 - a1 + 1
	b1 := rangeList[2]
	b2 := rangeList[3]
	bRange := b2 - b1 + 1
	c1 := rangeList[4]
	c2 := rangeList[5]
	cRange := c2 - c1 + 1
	d1 := rangeList[6]
	d2 := rangeList[7]
	dRange := d2 - d1 + 1
	total = aRange * bRange * cRange * dRange
	ips := make(chan string, total)
	for a := a1; a <= a2; a++ {
		for b := b1; b <= b2; b++ {
			for c := c1; c <= c2; c++ {
				for d := d1; d <= d2; d++ {
					addrString := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
					ips <- addrString
				}
			}
		}
	}
	return ips
}

func printUpdate() {
	if !done {
		fmt.Println("|==== statistics ====")
		fmt.Printf("| Time elapsed: %fs\n", time.Since(startTime).Seconds())
		fmt.Printf("| IPs Scanned: %d/%d (%.2f%%)\n\n", scanned, total, float64(scanned)/float64(total)*100)
		time.AfterFunc(5*time.Second, printUpdate)
	}
}

func parseFlags() {
	flag.BoolVar(&verbose, "v", false, "Usage: -v\nEnable verbose (print message for every IP scanned)")
	flag.StringVar(&outFile, "o", "", "Usage: -o [filename]\nSpecify file to write newline-separated list of IPs that responded to pings (default do not write to file)")
	flag.IntVar(&threadCount, "t", 8192, "Usage: -t [threadcount]\nspecify how many goroutines to use to ping IPs simultaneously (NOTE: threadcount must be >0)")
	flag.BoolFunc("help", "alias for -h", printHelp)
	flag.BoolFunc("h", "Print this message", printHelp)
	flag.Parse()
	if threadCount <= 0 {
		fmt.Println("threadCount is not greater than 0, defaulting to 8192")
		threadCount = 8192
	}
}

func printHelp(_ string) error {
	fmt.Printf("Usage of %s: %s [options] <port range>\n", os.Args[0], os.Args[0])
	fmt.Println("Port range should be in the following format: 0-255.0-255.0-255.0-255")
	flag.PrintDefaults()
	os.Exit(0)
	return nil
}

func saveToFile() {
	if outFile == "" {
		return
	}

	if _, err := os.Stat(outFile); err == nil {
		// file exists

		content, err := os.ReadFile(outFile)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", outFile, err)
			os.Exit(1)
		}
		if string(content) != "" {
			// file exists and is NOT empty, ask user if they want to overwrite or output to new file
			fmt.Printf("WARNING: file %s already exists and is not empty. Do you want to overwrite it? (y/N) > ", outFile)
			scan := goscan.NewScanner()
			inpt := strings.ToLower(scan.ReadLine())
			if inpt == "y" || inpt == "yes" {
				writeFile()
				return
			}

			fmt.Print("Would you like to output to a different file? (input filename, or nothing to cancel) > ")

			inpt = scan.ReadLine()
			if inpt == "" {
				return
			}
			outFile = inpt
			saveToFile()
			return
		}
		writeFile()
	} else if errors.Is(err, os.ErrNotExist) {
		writeFile()
	} else {
		// Schrodinger: file may or may not exist. See err for details.
		panic(err)
	}
	fmt.Printf("IPs successfully written to %s.\n", outFile)
}

func writeFile() {
	csvString := ""

	for len(resChan) > 0 {
		csvString += <-resChan + "\n"
	}

	bytes := []byte(csvString)
	err := os.WriteFile(outFile, bytes, 0644)
	if err != nil {
		panic(err)
	}

	username := os.Getenv("SUDO_USER")
	if username == "" {
		printErrString("Error: Could not get SUDO_USER\n")
	}

	usr, err := user.Lookup(username)
	if err != nil {
		printErrString("could not find user \"%s\": %v\n", username, err)
	}
	uid, _ := strconv.Atoi(usr.Uid)
	gid, _ := strconv.Atoi(usr.Gid)

	err = os.Chown(outFile, uid, gid)
}
