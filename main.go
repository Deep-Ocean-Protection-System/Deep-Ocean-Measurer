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

var pollTime = time.Second * 5

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
			fmt.Println("Running process without wait: ", prog.Path, prog.Arguments)
			err := utils.CallProcessWithoutWait(prog.Path, prog.Arguments)
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
	if len(processesStr) > 0 && string(processesStr[len(processesStr)-1]) == `,` {
		processesStr = processesStr[:len(processesStr)-1]
	}

	//fmt.Println("wait for 2 seconds before tracing")
	//time.Sleep(2 * time.Second)
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

		// find processes have exited
		for progIdx := range record.Programs {
			for procIdx := range record.Programs[progIdx].ProcInsts {
				isFoundProc := false
				for _, line := range lines {
					if strings.EqualFold(line[0], record.Programs[progIdx].Path) &&
						strings.EqualFold(line[2], record.Programs[progIdx].ProcInsts[procIdx].PID) {
						isFoundProc = true
						break
					}
				}
				if !isFoundProc {
					record.Programs[progIdx].ProcInsts[procIdx].EndTime = time.Now()
				}
			}
		}

		if len(lines) == 0 {
			fmt.Println("All processes are not running. Saving the latest record...")
			// store data to json file
			record.EndRecTime = time.Now()

			file, err := json.Marshal(record)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(config.OutPath, file, 0644)
			if err != nil {
				return err
			}
			fmt.Println("Save successfully!")
			return nil
		}

		//fmt.Println("Number of programs: ", len(record.Programs))
		{
			// extract and process data from csv
			for i := 2; i < len(lines); i++ {
				isSameProgram := false
				for j := 0; j < len(record.Programs); j++ {
					if record.Programs[j].Path == lines[i][0] {
						isSameProgram = true
						if len(record.Programs[j].ProcInsts) > 0 {
							isFoundProc := false
							for procIdx := range record.Programs[j].ProcInsts {
								if strings.EqualFold(lines[i][0], record.Programs[j].Path) && strings.EqualFold(lines[i][2],  record.Programs[j].ProcInsts[procIdx].PID) {
									record.Programs[j].ProcInsts[procIdx].CpuUsage = append(record.Programs[j].ProcInsts[procIdx].CpuUsage, lines[i][3])

									record.Programs[j].ProcInsts[procIdx].WorkingSetMemoryUsage = append(record.Programs[j].ProcInsts[procIdx].WorkingSetMemoryUsage, lines[i][4])

									isFoundProc = true
									break
								}
							}
							if !isFoundProc {
								// open the new process record
								record.Programs[j].ProcInsts = append(record.Programs[j].ProcInsts,
									data.NewProcessInstance(lines[i]))
							}
						} else {
							record.Programs[j].ProcInsts = append(record.Programs[j].ProcInsts,
								data.NewProcessInstance(lines[i]))
						}
						break
					}
				}
				if !isSameProgram {
					record.Programs = append(record.Programs, data.NewProgramEntity(lines[i]))
				}
			}

			// store data to json file
			record.EndRecTime = time.Now()

			file, err := json.Marshal(record)
			// file, err := json.MarshalIndent(record, " ", " ")
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
