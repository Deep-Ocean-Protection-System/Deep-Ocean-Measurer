package main

import (
	"domeasurer/data"
	"domeasurer/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akamensky/argparse"
)

var pollTime = time.Second * 1

func cleanUp() {
	// remove temp proc stat csv file
	os.Remove(data.TMP_PROC_STAT_CSV_FILE)
}

func traceProcesses(config data.TraceConfig) error {
	headers := []string{"Path", "ProcessName", "Id", "CPU", "WS"}

	var processesStr string
	for _, prog := range config.Programs {
		// launch each process if needed
		if prog.ForceRun {
			err := utils.CallProcessWDiffCtx(prog.Path, prog.Arguments)
			if err != nil {
				fmt.Println("cannot spawn", prog.Path, ":", err.Error())
			}
		}

		// construct string of process list
		progStr := prog.Path
		for i := len(progStr) - 1; i >= 0; i-- {
			if string(progStr[i]) == `\` || string(progStr[i]) == `/` {
				baseProgName := progStr[i+1:]
				processesStr += `"` + strings.TrimSuffix(baseProgName, filepath.Ext(baseProgName)) + `",`
				break
			}
		}
	}
	if string(processesStr[len(processesStr)-1]) == `,` {
		processesStr = processesStr[:len(processesStr)-1]
	}

	// get statistics about processes via powershell
	psCmd := `Get-Process -Name ` + processesStr + ` 2>$null | Select-Object ` + strings.Join(headers, ",")
	fmt.Println("powershell command: ", psCmd)

	startTime := time.Now()
	timeOut, err := time.ParseDuration(config.TimeOut)
	if err != nil {
		return err
	}
	record := data.Record{
		Programs:     config.Programs,
		StartRecTime: startTime,
	}
	// fmt.Println("time out: ", config.TimeOut)

	// tracing processes with timeout
	// if time out is zero, we have an endless tracing loop
	for config.TimeOut == "0" || timeOut > time.Since(startTime) {
		lines, err := utils.FetchDataByPowershell(psCmd, data.TMP_PROC_STAT_CSV_FILE)
		if err != nil {
			return err
		}
		// check if some or all process have exited
		if len(lines)-2 < len(processesStr) {
			// find processes have exited
			for _, prog := range config.Programs {
				progPath := prog.Path
				isFound := false
				for _, line := range lines {
					if strings.EqualFold(line[0], progPath) {
						isFound = true
						break
					}
				}
				if !isFound {
					// close the old process instance
					for i := 0; i < len(config.Programs); i++ {
						if config.Programs[i].Path == progPath {
							// only look for the last process instance
							if len(config.Programs[i].ProcInsts) > 0 && config.Programs[i].ProcInsts[len(config.Programs[i].ProcInsts)-1].EndTime.IsZero() {
								config.Programs[i].ProcInsts[len(config.Programs[i].ProcInsts)-1].EndTime = time.Now()
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
				for j := 0; j < len(config.Programs); j++ {
					if config.Programs[j].Path == lines[i][0] {
						isSameProgram = true
						if len(config.Programs[j].ProcInsts) > 0 {
							// only needs to check for the last process record
							lastProcIdx := len(config.Programs[j].ProcInsts) - 1
							if config.Programs[j].ProcInsts[lastProcIdx].PID == lines[i][2] {
								config.Programs[j].ProcInsts[lastProcIdx].CpuUsage = append(config.Programs[j].ProcInsts[lastProcIdx].CpuUsage, lines[i][3])

								config.Programs[j].ProcInsts[lastProcIdx].WorkingSetMemoryUsage = append(config.Programs[j].ProcInsts[lastProcIdx].WorkingSetMemoryUsage, lines[i][4])
								break
							} else {
								// the last process has exited
								// step 1: close the last process record
								config.Programs[j].ProcInsts[lastProcIdx].EndTime = time.Now()

								// step 2: open the new process record
								config.Programs[j].ProcInsts = append(config.Programs[j].ProcInsts,
									data.NewProcessInstance(lines[i]))
							}

						} else {
							config.Programs[j].ProcInsts = append(config.Programs[j].ProcInsts,
								data.NewProcessInstance(lines[i]))
						}
						break
					}
				}
				if !isSameProgram {
					config.Programs = append(config.Programs, data.NewProgramEntity(lines[i]))
				}
			}

			// store data to json file
			//file, err := json.Marshal(config.Programs)
			record.EndRecTime = time.Now()
			file, err := json.MarshalIndent(record, " ", " ")
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(config.OutPath, file, 0644)
			if err != nil {
				return err
			}
		}

		time.Sleep(pollTime)
	}
	return nil
}

func main() {
	parser := argparse.NewParser("Deep Ocean Measurer", "A quick tool for tracing process statistics")
	var progStrList *[]os.File = parser.FileList("p", "path", os.O_RDONLY, 0600, &argparse.Options{
		Help: "List of path for tracing",
	})
	var traceTime *string = parser.String("t", "time",
		&argparse.Options{
			Default: "0s",
			Help:    "Time for tracing. Use zero for endless tracing",
		})
	var outputFile *os.File = parser.File("o", "output-file", os.O_RDWR, 0600, &argparse.Options{
		Help: "Path of File to write recorded data",
	})
	var configFile *os.File = parser.File("f", "config-file", os.O_RDONLY, 0600, &argparse.Options{
		Help: "Path of Tracing Config File",
	})
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
	if configFile != nil {
		config, err = data.NewTraceConfig(configFile.Name())
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(config.OutPath); os.IsNotExist(err) {
			log.Fatal(errors.New("output path does not exist"))
		}
		if len(config.CheckpointPath) > 0 {
			if _, err := os.Stat(config.CheckpointPath); os.IsNotExist(err) {
				log.Fatal(errors.New("checkpoint path does not exist"))
			}
		}
	} else {
		if outputFile == nil {
			log.Fatal(errors.New("no output path has been provided"))
		}
		if checkpointFile != nil {
			config.CheckpointPath = checkpointFile.Name()
		}
		config.OutPath = outputFile.Name()
		config.TimeOut = *traceTime
		config.CheckpointPath = checkpointFile.Name()

		// construct program entity list
		if len(*progStrList) == 0 {
			log.Fatal(errors.New("empty program list"))
		}
		for _, progStr := range *progStrList {
			config.Programs = append(config.Programs,
				data.ProgramEntity{
					Path: progStr.Name(),
				})
		}
		//fmt.Printf("%+v\n", progList)
		//os.Exit(0)
	}

	// restore data from checkpoint
	if len(config.CheckpointPath) > 0 {
		jsonFile, err := os.Open(config.CheckpointPath)
		if err != nil {
			log.Fatal(err)
		}
		defer jsonFile.Close()
		jsonByte, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			log.Fatal(err)
		}

		var record data.Record
		err = json.Unmarshal(jsonByte, &record)
		if err != nil {
			log.Fatal(err)
		}
		// config.Programs = record.Programs
		// restore data for programs that existing in config and checkpoint
		for i := range config.Programs {
			for j := range record.Programs {
				if strings.EqualFold(config.Programs[i].Path, record.Programs[j].Path) && strings.EqualFold(config.Programs[i].Arguments, record.Programs[j].Arguments) {
					config.Programs[i].ProcInsts = record.Programs[j].ProcInsts
				}
			}
		}
		// fmt.Printf("%+v\n", config.Programs)
	}

	err = traceProcesses(config)
	if err != nil {
		fmt.Println(err)
	}
	cleanUp()
}
