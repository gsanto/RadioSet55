package main

// go install bitbucket.org/gsanto/ctl
// ctl
// go install bitbucket.org/gsanto/ctl && ctl
// test
// test2 gsanto@ubuntu:~/go/src/bitbucket.org/gsanto/webctl$
// webctl   ./run.sh

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tarm/goserial"
)

const (
	code1 = "08c0a8010cc0a80101aaaaaa0000"
	code2 = "08c0a8010cc0a801010005353535"
)

var serialName = flag.String("serialName", "/dev/ttyACM0", "serial port name (/dev/ttyACM0, COM3)")
var serialSpeed = flag.Int("serialSpeed", 115200, "serial port speed (115200)")
var port = flag.Int("port", 8080, "port on which to run the http server")

var serialSnd chan string
var serialRcv chan string

type Request struct {
	FrequencyA, FrequencyB float64
	Action                 string
}

type DataSet struct {
	Message                string
	FrequencyA, FrequencyB float64
	LastUpdateTime         int64
}

var dataSet DataSet

func Start() {
	http.Handle("/", http.FileServer(http.Dir("www/static")))
	http.HandleFunc("/data", MainHandler)

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", *port),
		ReadTimeout: 1 * time.Second,
	}
	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	SetFrequencyHandler(r)
	json.NewEncoder(w).Encode(dataSet)
}

func SetFrequencyHandler(r *http.Request) error {
	req := Request{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return err
	}

	bufferTx := ""
	switch req.Action {
	case "VFA":
		dataSet.FrequencyA = req.FrequencyA
		bufferTx = fmt.Sprintf("FA%012d;", uint64(req.FrequencyA*10)) // mult for MHz*10000000))
	case "VFB":
		dataSet.FrequencyB = req.FrequencyB
		bufferTx = fmt.Sprintf("FB%012d;", uint64(req.FrequencyB*10))
	case "FR0", "FR1", "SW09", "SW10", "SW26":
		bufferTx = req.Action
	case "FA?":
		bufferTx = "FA;"
	default:
		// Do not update LastUpdateTime.
		return nil
	}

	fmt.Printf("request: %v\n", req)
	dataSet.LastUpdateTime = time.Now().Unix()
	//bufferTx = "080000c0a8" + "ef" + DecodeString(string(ip1), err) + "/" + string(ipStr)

	if bufferTx != "" {
		log.Printf("TX buffer = %q\n", bufferTx)
		serialSnd <- bufferTx
		log.Printf("Sent\n")
	}

	return nil
}

func InitSerial() {
	serialRcv = make(chan string)
	serialSnd = make(chan string, 10000)

	c := &serial.Config{
		Name: *serialName,
		Baud: *serialSpeed,
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Printf("WARNING. Could not open serial port %s (%s)", *serialName, err)
		log.Printf("Try to change serial port)", *serialName, err)
		return
		// panic(err)
	}

	go func() {
		for {
			sndBuf := <-serialSnd
			s.Write([]byte(sndBuf))
		}
	}()

	go func() {

		rcvBufString := ""
		for {

			log.Printf("rcvBufString=%q\n", rcvBufString)

			separator := "\f\r"
			lines := strings.SplitN(rcvBufString, separator, 2)
			if len(lines) > 1 {
				trimLine := strings.TrimSpace(lines[0])
				if trimLine != "" {
					log.Printf("line=%q\n", trimLine)
					serialRcv <- trimLine
				}
				rcvBufString = lines[1]
			} else {
				rcvBuf := make([]byte, 1024)
				n, err := s.Read(rcvBuf)
				if err != nil {
					panic(err)
				}

				rcvBufString = rcvBufString + string(rcvBuf[:n])
			}
		}
	}()

	go func() {
		for {
			message := <-serialRcv
			dataSet.Message = message
		}
	}()
}

func main() {
	dataSet = DataSet{
		Message:        "-----",
		FrequencyA:     199,
		FrequencyB:     221,
		LastUpdateTime: time.Now().Unix(),
	}
	flag.Parse()
	InitSerial()
	Start()
}
