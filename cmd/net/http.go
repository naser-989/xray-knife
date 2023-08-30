package net

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/gocarina/gocsv"
	"github.com/spf13/cobra"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"xray-knife/speedtester/cloudflare"
	"xray-knife/utils"
	"xray-knife/utils/customlog"
	"xray-knife/xray"
)

var (
	configLinksFile   string
	outputFile        string
	outputType        string
	threadCount       uint16
	destURL           string
	httpMethod        string
	showBody          bool
	insecureTLS       bool
	verbose           bool
	sortedByRealDelay bool
	speedtest         bool
	speedtestAmount   uint32
)

var validConfigs []string
var validConfigsMu sync.Mutex

type result struct {
	ConfigLink    string  `csv:"link"`     // vmess://... vless//..., etc
	TLS           string  `csv:"tls"`      // none, tls, reality
	Delay         int64   `csv:"delay"`    // millisecond
	DownloadSpeed float32 `csv:"download"` // mbps
	UploadSpeed   float32 `csv:"upload"`   // mbps
}

type configResults []*result

func (cResults configResults) Len() int {
	return len(cResults)
}

func (cResults configResults) Less(i, j int) bool {
	if (cResults[i].Delay < cResults[j].Delay) && (cResults[i].DownloadSpeed >= cResults[j].DownloadSpeed) && (cResults[i].UploadSpeed >= cResults[j].UploadSpeed) {
		return true
	} else if cResults[i].Delay == cResults[j].Delay {
		return cResults[i].ConfigLink < cResults[j].ConfigLink
	}
	return false
}

func (cResults configResults) Swap(i, j int) {
	cResults[i], cResults[j] = cResults[j], cResults[i]
}

var confRes configResults

// HttpCmd represents the http command
var HttpCmd = &cobra.Command{
	Use:   "http",
	Short: "Examine config[s] real delay using http request",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if configLinksFile != "" {
			d := color.New(color.FgCyan, color.Bold)

			// Limit the number of concurrent workers
			semaphore := make(chan int, threadCount)

			// Wait for all workers to finish
			wg := sync.WaitGroup{}

			links := utils.ParseFileByNewline(configLinksFile)

			for i := 0; i < len(links); i++ {
				semaphore <- 1
				wg.Add(1)
				go func(configIndex int) {
					// Free the worker at the end
					defer func() {
						<-semaphore
						wg.Done()
					}()
					d.Printf("Config Number: %d\n", configIndex+1)
					parsed, err := xray.ParseXrayConfig(links[configIndex])
					if err != nil {
						customlog.Printf(customlog.Failure, "Couldn't parse the config!\n\n")
						return
						//os.Exit(1)
					}
					instance, err1 := xray.StartXray(parsed, verbose, insecureTLS)
					if err1 != nil {
						customlog.Printf(customlog.Failure, "Couldn't start the xray! : %v\n\n", err)
						return
					}

					var delay int64
					var downloadTime int64
					var uploadTime int64

					delay, _, err2 := xray.MeasureDelay(instance, time.Duration(15)*time.Second, showBody, destURL, httpMethod)
					if err2 != nil {
						customlog.Printf(customlog.Failure, "Config didn't respond!\n\n")
						return
						//os.Exit(1)
					}
					fmt.Printf("%v", parsed.DetailsStr())
					customlog.Printf(customlog.Success, "Real Delay: %dms\n", delay)

					if speedtest {
						downloadStartTime := time.Now()
						_, _, err := xray.CoreHTTPRequestCustom(instance, time.Duration(15)*time.Second, cloudflare.Speedtest.MakeDownloadHTTPRequest(false, speedtestAmount*1000))
						if err != nil {
							//customlog.Printf(customlog.Failure, "Download failed!\n")
							return
						} else {
							downloadTime = time.Since(downloadStartTime).Milliseconds()

							//customlog.Printf(customlog.Success, "Download took: %dms\n", downloadTime)
						}

						uploadStartTime := time.Now()
						_, _, err = xray.CoreHTTPRequestCustom(instance, time.Duration(15)*time.Second, cloudflare.Speedtest.MakeUploadHTTPRequest(false, speedtestAmount*1000))
						if err != nil {
							//customlog.Printf(customlog.Failure, "Upload failed!\n")
							return
						} else {
							uploadTime = time.Since(uploadStartTime).Milliseconds()

							//customlog.Printf(customlog.Success, "Upload took: %dms\n", uploadTime)
						}

					}
					fmt.Printf("\n")

					// Close xray conn after testing
					_ = instance.Close()

					r := &result{
						ConfigLink:    links[configIndex],
						TLS:           parsed.ConvertToGeneralConfig().TLS,
						Delay:         delay,
						DownloadSpeed: (float32((speedtestAmount*1000)*8) / float32(uint32(downloadTime)/1000.0)) / 1000000.0,
						UploadSpeed:   (float32((speedtestAmount*1000)*8) / float32(uint32(uploadTime)/1000.0)) / 1000000.0,
					}
					validConfigsMu.Lock()
					confRes = append(confRes, r)
					validConfigsMu.Unlock()
					return
				}(i)
			}
			// Wait for all goroutines to finish
			wg.Wait()

			// Close semaphore channel
			close(semaphore)

			// Sort configs based on their delay
			if sortedByRealDelay {
				sort.Sort(confRes)
			}
			for _, v := range confRes {
				validConfigs = append(validConfigs, v.ConfigLink)
			}

			if outputType == "txt" {
				// Save configs
				err := utils.WriteIntoFile(outputFile, []byte(strings.Join(validConfigs, "\n\n")))
				if err != nil {
					customlog.Printf(customlog.Failure, "Saving configs failed due to the error: %v\n", err)
					os.Exit(1)
				}
			} else if outputType == "csv" {
				if outputFile == "valid.txt" {
					outputFile = "valid.csv"
				}
				//writer := csv.NewWriter(f)
				//defer writer.Flush()

				//writer.Write(headers)
				//for _, row := range rows {
				//	writer.Write(row)
				//}
				out, err := gocsv.MarshalString(&confRes)
				if err != nil {
					customlog.Printf(customlog.Failure, "Saving configs failed due to the error: %v\n", err)
					os.Exit(1)
				}
				err = utils.WriteIntoFile(outputFile, []byte(out))
				if err != nil {
					customlog.Printf(customlog.Failure, "Saving configs failed due to the error: %v\n", err)
					os.Exit(1)
				}
			}

			customlog.Printf(customlog.Finished, "A total of %d configurations have been saved to %s\n", len(validConfigs), outputFile)
		} else {
			parsed, err := xray.ParseXrayConfig(configLink)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
				os.Exit(1)
			}

			fmt.Println("\n" + parsed.DetailsStr())

			instance, err := xray.StartXray(parsed, verbose, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
				os.Exit(1)
				return
			}

			delay, _, err := xray.MeasureDelay(instance, time.Duration(15)*time.Second, showBody, destURL, httpMethod)
			if err != nil {
				customlog.Printf(customlog.Failure, "Config didn't respond!")
				os.Exit(1)
			}
			customlog.Printf(customlog.Success, "Real Delay: %dms\n", delay)
			if speedtest {
				downloadStartTime := time.Now()
				_, _, err := xray.CoreHTTPRequestCustom(instance, time.Duration(15)*time.Second, cloudflare.Speedtest.MakeDownloadHTTPRequest(false, speedtestAmount*1000))
				if err != nil {
					customlog.Printf(customlog.Failure, "Download failed!\n")
					//return
				} else {
					downloadTime := time.Since(downloadStartTime).Milliseconds()

					customlog.Printf(customlog.Success, "Downloaded %dKB - Speed: %d mbps\n", speedtestAmount, (float32((speedtestAmount*1000)*8)/float32(uint32(downloadTime)/1000.0))/1000000.0)
				}

				uploadStartTime := time.Now()
				_, _, err = xray.CoreHTTPRequestCustom(instance, time.Duration(15)*time.Second, cloudflare.Speedtest.MakeUploadHTTPRequest(false, speedtestAmount*1000))
				if err != nil {
					customlog.Printf(customlog.Failure, "Upload failed!\n")
					//return
				} else {
					uploadTime := time.Since(uploadStartTime).Milliseconds()

					customlog.Printf(customlog.Success, "Uploaded %dKB - Speed: %d mbps\n", speedtestAmount, (float32((speedtestAmount*1000)*8)/float32(uint32(uploadTime)/1000.0))/1000000.0)
				}

			}
			//fmt.Printf("%s: %sms\n", color.RedString("Real delay"), color.YellowString(strconv.Itoa(int(delay))))
		}

	},
}

func init() {
	HttpCmd.Flags().StringVarP(&configLink, "config", "c", "", "The xray config link")
	HttpCmd.Flags().StringVarP(&configLinksFile, "file", "f", "", "Read config links from a file")
	HttpCmd.Flags().Uint16VarP(&threadCount, "thread", "t", 5, "Number of threads to be used for checking links from file")
	HttpCmd.Flags().StringVarP(&destURL, "url", "u", "https://google.com/", "The url to test config")
	HttpCmd.Flags().StringVarP(&httpMethod, "method", "m", "GET", "Http method")
	HttpCmd.Flags().BoolVarP(&showBody, "body", "b", false, "Show response body")
	HttpCmd.Flags().BoolVarP(&insecureTLS, "insecure", "e", false, "Insecure tls connection (fake SNI)")
	HttpCmd.Flags().BoolVarP(&speedtest, "speedtest", "p", false, "Speed test with speed.cloudflare.com")
	HttpCmd.Flags().Uint32VarP(&speedtestAmount, "amount", "a", 10000, "Download and upload amount (KB) default: 10000")
	HttpCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose xray-core")
	HttpCmd.Flags().StringVarP(&outputType, "type", "x", "txt", "Output type (csv, txt)")
	HttpCmd.Flags().StringVarP(&outputFile, "out", "o", "valid.txt", "Output file for valid config links")
	HttpCmd.Flags().BoolVarP(&sortedByRealDelay, "sort", "s", true, "Sort config links by their delay (fast to slow)")
}
