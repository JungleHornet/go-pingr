# pingr
Simple command-line tool to ping a range of IPs, written in go 

### Usage: (`pingr help`)
```
Usage of pingr: pingr [options] <port range>
Port range should be in the following format: 0-255.0-255.0-255.0-255
  -c int
        Usage: -c [count]
        specify how many times to ping every IP address (default 1)
  -h    Print this message
  -help
        alias for -h
  -o string
        Usage: -o [filename]
        Specify file to write newline-separated list of IPs that responded to pings (default do not write to file)
  -t int
        Usage: -t [threadcount]
        specify how many goroutines to use to ping IPs simultaneously (NOTE: threadcount must be >0) (default 8192)
  -v    Usage: -v
        Enable verbose (print message for every IP scanned)
```
