package data

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

type (
	ProgramEntity struct {
		Path      string            `json:"program_path"`
		Arguments string            `json:"program_args"`
		ForceRun  bool              `json:"force_run"`
		ProcInsts []ProcessInstance `json:"process_instances"`
	}

	ProcessInstance struct {
		Name                  string    `json:"process_name"`
		PID                   string    `json:"pid"`
		CpuUsage              []string  `json:"cpu_usage"`
		WorkingSetMemoryUsage []string  `json:"working_set_memory_usage"`
		StartTime             time.Time `json:"start_time"`
		EndTime               time.Time `json:"end_time"`
	}

	TraceConfig struct {
		Programs       []ProgramEntity `json:"programs"`
		CheckpointPath string          `json:"checkpoint_path"`
		OutPath        string          `json:"out_path"`
		TimeOut        string          `json:"time_out"`
	}

	Record struct {
		Programs     []ProgramEntity `json:"programs"`
		StartRecTime time.Time       `json:"start_rec_time"`
		EndRecTime   time.Time       `json:"end_rec_time"`
	}
)

func NewProgramEntity(dataList []string) ProgramEntity {
	return ProgramEntity{
		Path: dataList[0],
		ProcInsts: []ProcessInstance{
			NewProcessInstance(dataList),
		},
	}
}

func NewProcessInstance(dataList []string) ProcessInstance {
	return ProcessInstance{
		Name:                  dataList[1],
		PID:                   dataList[2],
		CpuUsage:              []string{dataList[3]},
		WorkingSetMemoryUsage: []string{dataList[4]},
		StartTime:             time.Now(),
	}
}

func NewTraceConfig(configPath string) (TraceConfig, error) {
	var traceConfig TraceConfig
	jsonFile, err := os.Open(configPath)
	if err != nil {
		return traceConfig, err
	}
	defer jsonFile.Close()
	jsonByte, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return traceConfig, err
	}

	err = json.Unmarshal(jsonByte, &traceConfig)
	if err != nil {
		return traceConfig, err
	}
	return traceConfig, err
}
