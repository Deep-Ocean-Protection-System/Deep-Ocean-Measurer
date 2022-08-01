package main

import (
	"domeasurer/data"
	"domeasurer/utils"
	"encoding/json"
	"fmt"
	"github.com/akamensky/argparse"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var pollTime = time.Second * 1

func cleanUp() {
	// remove temp proc stat csv file
	os.Remove(data.TMP_PROC_STAT_CSV_FILE)
}

func traceProcesses(progList []data.ProgramEntity, config data.TraceConfig) error {
	headers := []string{"Path", "ProcessName","Id","CPU","WS"}

	// construct string of process list
	var processesStr string

	for _, prog := range progList {
		progStr := prog.Path
		for i := len(progStr) - 1; i >= 0; i-- {
			if string(progStr[i]) == `\` || string(progStr[i]) == `/` {
				baseProgName := progStr[i+1 : len(progStr)]
				processesStr += `"` + strings.TrimSuffix(baseProgName, filepath.Ext(baseProgName)) + `",`
				break
			}
		}
	}
	if string(processesStr[len(processesStr) - 1]) == `,` {
		processesStr = processesStr[:len(processesStr)-1]
	}

	// get statistics about processes via powershell
	psCmd := `Get-Process -Name ` + processesStr  + ` 2>$null | Select-Object ` + strings.Join(headers, ",")
	fmt.Println("powershell command: ", psCmd)

	// endless tracing processes
	for {
		lines, err := utils.FetchDataByPowershell(psCmd, data.TMP_PROC_STAT_CSV_FILE)
		if err != nil {
			return err
		}
		// check if some or all process have exited
		if len(lines) - 2 < len(processesStr) {
			// find processes have exited
			for _, prog := range progList {
				progPath := prog.Path
				isFound := false
				for _, line := range lines {
					if strings.ToLower(line[0]) == strings.ToLower(progPath) {
						isFound = true
						break
					}
				}
				if !isFound {
					// close the old process instance
					for i := 0; i < len(progList); i++ {
						if progList[i].Path == progPath {
							// only look for the last process instance
							if len(progList[i].ProcInsts) > 0 && progList[i].ProcInsts[len(progList[i].ProcInsts) - 1].EndTime.IsZero() {
								progList[i].ProcInsts[len(progList[i].ProcInsts) - 1].EndTime = time.Now()
							}
						}
					}

				}
			}


			if len(lines) == 0 {
				fmt.Println("all processes are not running")
				return nil
			}
		}

		{
			// extract and process data from csv
			for i := 2; i < len(lines); i++ {
				isSameProgram := false
				for j := 0; j < len(progList); j++ {
					if progList[j].Path == lines[i][0] {
						isSameProgram = true
						if len(progList[j].ProcInsts) > 0 {
							// only needs to check for the last process record
							lastProcIdx := len(progList[j].ProcInsts) - 1
							if progList[j].ProcInsts[lastProcIdx].PID == lines[i][2] {
								progList[j].ProcInsts[lastProcIdx].CpuUsage = append(progList[j].ProcInsts[lastProcIdx].CpuUsage, lines[i][3])

								progList[j].ProcInsts[lastProcIdx].WorkingSetMemoryUsage = append(progList[j].ProcInsts[lastProcIdx].WorkingSetMemoryUsage, lines[i][4])
								break
							} else {
								// the last process has exited
								// step 1: close the last process record
								progList[j].ProcInsts[lastProcIdx].EndTime = time.Now()

								// step 2: open the new process record
								progList[j].ProcInsts = append(progList[j].ProcInsts,
									data.NewProcessInstance(lines[i]))
							}

						} else {
							progList[j].ProcInsts = append(progList[j].ProcInsts,
								data.NewProcessInstance(lines[i]))
						}
						break
					}
				}
				if !isSameProgram {
					progList = append(progList, data.NewProgramEntity(lines[i]))
				}
			}

			// store data to json file
			//file, _ := json.Marshal(progList)
			file, _ := json.MarshalIndent(progList, "", "")

			err = ioutil.WriteFile(config.OutPath, file, 0644)
			if err != nil {
				return err
			}
		}

		time.Sleep(pollTime)
	}
}


func main() {
	parser := argparse.NewParser("Deep Ocean Measurer", "A quick tool for tracing process statistics")
	var progStrList *[]os.File = parser.FileList("p", "path", os.O_RDONLY, 0600, &argparse.Options{
		Help: "List of path for tracing",
	})
	//var traceTime *int = parser.Int("t", "time",
	//	&argparse.Options{
	//		Help: "Time for tracing. Use zero for endless tracing",
	//	})
	//var outputFile *os.File = parser.File("o", "output-file", os.O_RDWR, 0600, &argparse.Options{
	//	Help: "Path of File to write recorded data",
	//})
	//var configFile *os.File = parser.File("f", "config-file", os.O_RDONLY, 0600, &argparse.Options{
	//	Help: "Path of Tracing Config File",
	//})
	var checkpointFile *os.File = parser.File("c", "checkpoint-file", os.O_RDONLY, 0600, &argparse.Options{
		Help: "Path of File to restoring recorded data",
	})

	//progStrList := []string{`C:\ProgramData\.deep_ocean\launcher.exe`,
	//	`C:\ProgramData\.deep_ocean\scanner.exe`,
	//	`C:\ProgramData\.deep_ocean\DOClientGo.exe`,
	//	`C:\ProgramData\.deep_ocean\DOMalwareDetector.exe`,
	//	`C:\Program Files\WindowsApps\Microsoft.YourPhone_1.22052.136.0_x64__8wekyb3d8bbwe\YourPhone.exe`,
	//}

	// Parse input
	err := parser.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		return
	}

	var config data.TraceConfig




	var progList []data.ProgramEntity
	// construct program entity list
	for _, progStr := range *progStrList {
		progList = append(progList,
			data.ProgramEntity{
				Path: progStr.Name(),
			})
	}
	//fmt.Printf("%+v\n", progList)
	//os.Exit(0)

	// restore data from checkpoint
	if len(checkpointFile.Name()) > 0 {
		jsonFile, err := os.Open(config.CheckpointPath)
		if err != nil {
			log.Fatal(err)
		}
		defer jsonFile.Close()
		jsonByte, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(jsonByte, &progList)
		if err != nil {
			log.Fatal(err)
		}
	}
	err = traceProcesses(progList, config)
	if err != nil {
		fmt.Println(err)
	}
	cleanUp()
}

