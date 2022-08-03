package utils

import (
	"encoding/csv"
	"os"
	"os/exec"
	"syscall"
)

func CallProcess(processName string, getOutput bool, args ...string) (isOK bool, res string, err error) {
	var isSuccess bool = true
	var strResult string = ""

	var strArg string = ""

	for _, arg := range args {
		strArg += arg
		strArg += " "
	}
	cmd := exec.Command(processName, strArg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	result, err := cmd.Output()
	if err != nil {
		isSuccess = false
	} else if getOutput {
		if len(result) == 0 {
			isSuccess = false
		} else {
			strResult = string(result)
		}
	}
	return isSuccess, strResult, err
}

func CallProcessWithoutWait(processName string, args ...string) error {
	var strArg string = ""

	for _, arg := range args {
		strArg += arg
		strArg += " "
	}
	if len(strArg) > 0 && string(strArg[len(strArg)-1]) == ` ` {
		strArg = strArg[:len(strArg)-1]
	}
	// fmt.Println("Path: ", processName)
	// fmt.Println("Args: ", strArg)
	cmd := exec.Command(processName, strArg)
	//cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func CallProcessWDiffCtx(processName string, args ...string) error {
	var strArg string = " "

	for _, arg := range args {
		strArg += arg
		strArg += " "
	}
	// fmt.Println("Path: ", processName)
	// fmt.Println("Args: ", strArg)
	cmd := exec.Command("C:\\Windows\\System32\\cmd.exe", "/c", "start", "/B", processName, strArg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func FetchDataByPowershell(psCmd string, csvFilePath string) (res [][]string, err error) {
	var lines [][]string
	// change the file name into a varriable for easier managing
	CallProcess("powershell.exe", false, psCmd+" | Export-CSV -Path "+csvFilePath)
	// this will export information to a file "exported.csv" for parsing

	// read csv file
	csvList, err := os.Open(csvFilePath)
	if err != nil {
		return lines, err
	}

	csvReader := csv.NewReader(csvList)
	csvReader.FieldsPerRecord = -1 // ignore the number of fields
	lines, err = csvReader.ReadAll()
	if err != nil {
		return lines, err
	}
	csvList.Close()
	return lines, err
}
