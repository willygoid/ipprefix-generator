package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

const (
	workerCount   = 100 // Number of concurrent workers
	batchSize     = 1000 // Number of IPs to write at once
	ipRangeBuffer = 100000 // Buffer size for IP ranges
)

func main() {
	// Open the input file
	file, err := os.Open("ipprefix.txt")
	if err != nil {
		fmt.Println("Error opening input file:", err)
		return
	}
	defer file.Close()

	// Create or truncate the output file
	outputFile, err := os.Create("ipaddress.txt")
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	var wg sync.WaitGroup

	// Channel to communicate IP ranges and control worker concurrency
	ipRangeChan := make(chan string, ipRangeBuffer)
	workerChan := make(chan struct{}, workerCount)

	// Start a goroutine to write to the file
	go func() {
		var batch []string
		for ipRange := range ipRangeChan {
			batch = append(batch, ipRange)
			if len(batch) >= batchSize {
				if _, err := writer.WriteString(join(batch)); err != nil {
					fmt.Println("Error writing to output file:", err)
				}
				batch = nil // Reset batch
			}
		}
		// Write any remaining IPs in the batch
		if len(batch) > 0 {
			if _, err := writer.WriteString(join(batch)); err != nil {
				fmt.Println("Error writing to output file:", err)
			}
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		prefix := scanner.Text()
		wg.Add(1)

		// Acquire a worker
		workerChan <- struct{}{}

		go func(prefix string) {
			defer wg.Done()
			defer func() { <-workerChan }() // Release worker

			// Print the current IP prefix being processed
			fmt.Println("Processing prefix:", prefix)

			_, ipNet, err := net.ParseCIDR(prefix)
			if err != nil {
				fmt.Println("Error parsing prefix:", err)
				return
			}

			// Generate IP range and send to channel
			if err := generateIPRange(ipNet, ipRangeChan); err != nil {
				fmt.Println("Error generating IP range:", err)
			}
		}(prefix)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading lines:", err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(ipRangeChan)

	// Flush the writer buffer to ensure all data is written to the file
	if err := writer.Flush(); err != nil {
		fmt.Println("Error flushing writer:", err)
	}
}

// join joins a slice of strings into a single string.
func join(batch []string) string {
	result := ""
	for _, ip := range batch {
		result += ip
	}
	return result
}

// generateIPRange generates IPs in the given CIDR block and sends them to the channel.
func generateIPRange(ipNet *net.IPNet, ipRangeChan chan<- string) error {
	startIP := ipNet.IP.Mask(ipNet.Mask)
	endIP := make(net.IP, len(startIP))

	// Calculate the end IP address by setting all host bits to 1
	for i := range endIP {
		endIP[i] = startIP[i] | ^ipNet.Mask[i]
	}

	// Write the range of IPs to the channel
	for ip := startIP; !ip.Equal(endIP); incrementIP(ip) {
		ipRangeChan <- ip.String() + "\n"
	}
	// Write the end IP
	ipRangeChan <- endIP.String() + "\n"

	return nil
}

// incrementIP increments the given IP address.
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}
