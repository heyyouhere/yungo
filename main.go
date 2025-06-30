package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"flag"

	"golang.org/x/crypto/ssh"
)

type Status struct{
    Id string
    Names []string
    Created int
    State string
    Status string
	Image string
	Command string

}

func (s *Status) Display(showStopped bool){
	var sb strings.Builder
	const (
		red   = "\033[31m" // Red color
		green = "\033[32m" // Green color
		circle = "●"
		reset = "\033[0m"  // Reset to default color
	)
	if s.State == "running"{
		sb.WriteString(green)
	} else {
		if !showStopped{
			return
		}
		sb.WriteString(red)
	}
	sb.WriteString(circle)
	sb.WriteString(reset)
	sb.WriteString(fmt.Sprintf(" %s\t", s.Names[0]))
	sb.WriteString(fmt.Sprintf("%s\t", s.Status))
	sb.WriteString(fmt.Sprintf("%s\t", s.Image))
	fmt.Printf("%s\n", sb.String())
}


// TODO this is bad abstraction, rearrange
type Dock struct{
	Host string
	Port string
	Username string
}

func (d *Dock) Connect(privateKeyPath string) (*ssh.Client, error){
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{ User: d.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%s", d.Host, d.Port)
	remoteClient, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}
	return remoteClient, err
}


func (d *Dock) GetStatus(remoteClient *ssh.Client, socketPath string) ([]Status, error){
	// TODO: this is going to be run in a timeout loop,
	// so it is probably better idea to store remoteConn,
	// rather than remoteClient
	remoteConn, err := remoteClient.Dial("unix", socketPath)
	defer remoteConn.Close()
	request := "GET /containers/json?all=true HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	_, err = remoteConn.Write([]byte(request))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 1024*1024*1024)
	_, err = remoteConn.Read(buf)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(buf)

	res, err := http.ReadResponse(bufio.NewReader(buffer), nil)
	if err != nil{
		return nil, err
	}

	var statuses []Status
	bodyBuff := make([]byte, 1024*1024)
	n, _ := res.Body.Read(bodyBuff)
	err = json.Unmarshal(bodyBuff[:n], &statuses)
	if err != nil {
		return nil, err
	}
	return statuses, nil
}

func (d *Dock) GetLogs(remoteClient *ssh.Client, socketPath string, container_id string) error{
	// TODO: this function does not return logs, since the endpoint writes as
	// Content-Type: application/vnd.docker.multiplexed-stream
	// And I dont know how to handle it in golang

	remoteConn, err := remoteClient.Dial("unix", socketPath)
	defer remoteConn.Close()
	request := fmt.Sprintf("GET /containers/%s/logs?stdout=true&tail=40 HTTP/1.1\r\nHost: localhost\r\n\r\n", container_id)
	_, err = remoteConn.Write([]byte(request))
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*1024)
	n, err := remoteConn.Read(buf)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(buf[:n]))

	fmt.Printf("Reading chunk-size\n")
	n1, err := remoteConn.Read(buf)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", string(buf[n:n1]))
	os.Exit(0)

	// res, err := http.ReadResponse(bufio.NewReader(buffer), nil)
	// if err != nil{
	// 	return err
	// }
	// bodyBuff := make([]byte, 1024*1024)
	// res.Body.Read(bodyBuff)
	// fmt.Printf("%s\n", string(bodyBuff))
	return nil
}



func main() {
	var (
		showStopped = flag.Bool("s", false, "Display stopped containers")
		privateKey = flag.String("k", "", "Path to private key")
	)
	flag.Parse()
	privateKeyPath := *privateKey
	if len(privateKeyPath) == 0{
		fmt.Printf("No private key path provided.\n")
		fmt.Printf("Usage:\n%s [OPTIONS] -k PATH/TO/PRIVATE_KEY\n", os.Args[0])
		os.Exit(1)
	}
	hostsFile, err := os.Open("hosts")
	if err != nil{
		fmt.Printf("Could not open hosts file\n")
	}
	hostsBytes, err := io.ReadAll(hostsFile)
	if err != nil{
		fmt.Printf("Could not read bytes from hosts file\n")
	}
	docks := []Dock{}
	err = json.Unmarshal(hostsBytes, &docks)
	if err != nil{
		fmt.Printf("Could not unmarshal, %s", err)
	}

	remoteSocketPath := "/var/run/docker.sock"
	for _, dock := range docks{
		fmt.Printf("--------------------------------------------------\n")
		fmt.Printf("Dock %s@%s\n", dock.Username, dock.Host)
		client, err := dock.Connect(privateKeyPath)
		defer client.Close()
		if err != nil{
			fmt.Printf("Could not connect to dock %s, %s\n", dock.Host, err)
			continue
		}
		statuses, err := dock.GetStatus(client, remoteSocketPath)
		if err != nil{
			fmt.Printf("Could not GetStatus of Dock %s, %s\n", dock.Host, err)
			continue
		}
		sort.Slice(statuses, func(i, j int) bool {
			if statuses[i].State == "running"{
				return true
			}
			if statuses[j].State == "running"{
				return false
			}
			return true
		})
		for _, s := range statuses{
			s.Display(*showStopped)
			// dock.GetLogs(client, remoteSocketPath, s.Id)
		}
	}
}
