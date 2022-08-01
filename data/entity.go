package data

import "time"

type (
	ProgramEntity struct {
		Path                  string   `json:"program_path"`
		ProcInsts []ProcessInstance `json:"process_instances"`
	}

	ProcessInstance struct {
		Name                  string   `json:"process_name"`
		PID                   string   `json:"pid"`
		CpuUsage              []string `json:"cpu_usage"`
		WorkingSetMemoryUsage []string `json:"working_set_memory_usage"`
		StartTime time.Time `json:"start_time"`
		EndTime time.Time `json:"end_time"`
	}

	TraceConfig struct {
		CheckpointPath string `json:"checkpoint_path"`
		OutPath string `json:"out_path"`
		TimeOut time.Duration `json:"time_out"`
	}
)

func NewProgramEntity (dataList []string) ProgramEntity {
	return ProgramEntity{
		Path: dataList[0],
		ProcInsts: []ProcessInstance {
			NewProcessInstance(dataList),
		},
	}
}

func NewProcessInstance(dataList []string) ProcessInstance {
	return ProcessInstance{
		Name: dataList[1],
		PID: dataList[2],
		CpuUsage: []string{dataList[3]},
		WorkingSetMemoryUsage: []string{dataList[4]},
		StartTime: time.Now(),
	}
}
