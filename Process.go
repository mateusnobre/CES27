package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TIMEOUT = 5

type Process struct {
	id         int64
	connection *net.UDPConn
	address    *net.UDPAddr
}

type ProcessState int64

const (
	ProcessStateReleased ProcessState = iota
	ProcessStateWanted
	ProcessStateHeld
)

type CounterWithMutex struct {
	count int
	mutex *sync.Mutex
}

type CurrentProcess struct {
	id                 int64
	clock              CounterWithMutex
	state              ProcessState
	address            *net.UDPAddr
	receiver           *net.UDPConn
	linkedProcesses    []*Process
	sharedResource     *net.UDPConn
	receivedReplyCount CounterWithMutex
	replyQueue         []int64
}

func (s ProcessState) String() string {
	switch s {
	case ProcessStateReleased:
		return "Sai da CS"
	case ProcessStateWanted:
		return "Aguardando acesso a CS"
	case ProcessStateHeld:
		return "Entrei na CS"
	}
	return ""
}

/* A simple function to verify error */
func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func IncrementCounter(counter *CounterWithMutex) {
	counter.mutex.Lock()
	counter.count++
	counter.mutex.Unlock()
}

func sendRequestsToAccessCS(CurrentProcess *CurrentProcess) {
	for _, linkedProcess := range CurrentProcess.linkedProcesses {
		fmt.Println(fmt.Sprintf("Send request from %d to %d, to port %d from %d", CurrentProcess.id, linkedProcess.id, linkedProcess.address.Port, CurrentProcess.address.Port))
		buf := []byte(fmt.Sprintf("Requesting CS at %d to %d process", CurrentProcess.clock.count, CurrentProcess.id))
		_, err := linkedProcess.connection.Write(buf)
		CheckError(err)
	}
}

func replyProcess(CurrentProcess *CurrentProcess, processId int64) {
	for _, linkedProcess := range CurrentProcess.linkedProcesses {
		if linkedProcess.id == processId {
			fmt.Println(fmt.Sprintf("Send reply from %d to %d, to port %d from %d", CurrentProcess.id, linkedProcess.id, linkedProcess.address.Port, CurrentProcess.address.Port))
			buf := []byte(fmt.Sprintf("Replying CS at %d to %d and port %d ", CurrentProcess.clock.count, processId, linkedProcess.address.Port))
			_, err := linkedProcess.connection.Write(buf)
			CheckError(err)
		}
	}
}

func DoClientJob(CurrentProcess *CurrentProcess, CurrentProcessPort int64, ConnWithSharedResource *net.UDPConn) {
	if CurrentProcess.state == ProcessStateHeld {
		fmt.Println("IGNORED! Process", CurrentProcess.id, "is already on CS")
	} else if CurrentProcess.state == ProcessStateWanted {
		fmt.Println("IGNORED! Process", CurrentProcess.id, "is waiting to enter CS")
	} else {
		fmt.Println("Accessing Critical Section")
		CurrentProcess.state = ProcessStateWanted
		IncrementCounter(&CurrentProcess.clock)
		sendRequestsToAccessCS(CurrentProcess)
		fmt.Println("Waiting for replies...")
		for CurrentProcess.receivedReplyCount.count < len(CurrentProcess.linkedProcesses) {
		}
		CurrentProcess.state = ProcessStateHeld
		fmt.Println("Acessing CS")
		IncrementCounter(&CurrentProcess.clock)
		buf := []byte(fmt.Sprintf("Acessing CS at %d on port %d", CurrentProcess.clock.count, CurrentProcessPort))
		_, err := ConnWithSharedResource.Write(buf)
		CheckError(err)
		time.Sleep(TIMEOUT * time.Second)
		CurrentProcess.state = ProcessStateReleased
		for _, replyProcessId := range CurrentProcess.replyQueue {
			replyProcess(CurrentProcess, replyProcessId)
		}
		fmt.Println("Releasing CS")
	}
}

func updateClock(CurrentProcess *CurrentProcess, receivedClock int) {
	CurrentProcess.clock.mutex.Lock()
	if CurrentProcess.clock.count < receivedClock {
		CurrentProcess.clock.count = receivedClock + 1
	} else {
		CurrentProcess.clock.count = CurrentProcess.clock.count + 1
	}
	CurrentProcess.clock.mutex.Unlock()
}

func DoServerJob(CurrentProcess *CurrentProcess) {
	buf := make([]byte, 4096)

	for {
		_, _, err := CurrentProcess.receiver.ReadFromUDP(buf)
		words := strings.Fields(string(buf))

		CheckError(err)
		requestType := words[0]
		requestTime, err := strconv.Atoi(words[3])
		CheckError(err)
		requestProcessId, err := strconv.ParseInt(words[5], 10, 64)
		CheckError(err)
		fmt.Println("Received", requestType, "from", requestProcessId, "at", requestTime)
		if requestType == "Requesting" {
			if CurrentProcess.state == ProcessStateHeld || (CurrentProcess.state == ProcessStateWanted && CurrentProcess.clock.count < requestTime) {
				CurrentProcess.replyQueue = append(CurrentProcess.replyQueue, requestProcessId)
				fmt.Println("Queueing request at", CurrentProcess.clock.count, "from", requestProcessId)
			} else {
				replyProcess(CurrentProcess, requestProcessId)
			}
		} else if requestType == "Replying" {
			IncrementCounter(&CurrentProcess.receivedReplyCount)
			fmt.Println("Increment Reply Count")
		}
		updateClock(CurrentProcess, requestTime)
	}
}

func main() {
	argsWithoutProg := os.Args[1:]
	CurrentProcessId, err := strconv.ParseInt(argsWithoutProg[0], 10, 64)
	CheckError(err)
	CurrentProcessPort := CurrentProcessId + 10001

	/* Lets prepare a address at any address at port 10001*/
	SharedResourceAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	CheckError(err)

	CurrentProcessAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	CheckError(err)
	/* Listen to replies from other processes */
	CurrentProcessAddrToListen, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", CurrentProcessPort))
	CheckError(err)

	CurrentProcessConn, err := net.ListenUDP("udp", CurrentProcessAddrToListen)
	CheckError(err)

	defer CurrentProcessConn.Close()
	/* Make connection to send messages to Shared Resource/Critical Section */
	ConnWithSharedResource, err := net.DialUDP("udp", CurrentProcessAddr, SharedResourceAddr)
	CheckError(err)

	// defer ConnWithSharedResource.Close()

	CheckError(err)
	CurrentProcess := CurrentProcess{
		id:                 CurrentProcessId,
		clock:              CounterWithMutex{count: 0, mutex: &sync.Mutex{}},
		state:              ProcessStateReleased,
		address:            CurrentProcessAddrToListen,
		receiver:           CurrentProcessConn,
		sharedResource:     ConnWithSharedResource,
		receivedReplyCount: CounterWithMutex{count: 0, mutex: &sync.Mutex{}},
		replyQueue:         make([]int64, 0, 3),
	}

	processes := make([]*Process, 0, len(argsWithoutProg)-1)
	for i := 1; i < len(argsWithoutProg); i++ {
		port, _ := strconv.ParseInt(argsWithoutProg[i][1:], 10, 64)
		id := port - 10001
		if id != CurrentProcessId {
			ProcessAddress, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.01:%d", port))
			CheckError(err)
			ProcessConn, err := net.DialUDP("udp", CurrentProcessAddr, ProcessAddress)
			CheckError(err)
			process := Process{id: id, connection: ProcessConn, address: ProcessAddress}
			processes = append(processes, &process)
		}
	}
	CurrentProcess.linkedProcesses = processes

	go DoServerJob(&CurrentProcess)

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Enter text: ")
		msg, _ := reader.ReadString('\n')
		msg = msg[:len(msg)-1]
		if msg == "x" {
			go DoClientJob(&CurrentProcess, CurrentProcessPort, ConnWithSharedResource)
		} else if msg == string(CurrentProcessId) {
			fmt.Println("Increment Clock\n")
			IncrementCounter(&CurrentProcess.clock)
		}
		buf := []byte(fmt.Sprintf("%s at time %d on port %d", msg, CurrentProcess.clock.count, CurrentProcessPort))
		_, err := ConnWithSharedResource.Write(buf)
		if err != nil {
			fmt.Println(msg, err)
		}
	}
}
