package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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

func (s *Status) Display(){
	fmt.Printf("    Name    : %s\n",   s.Names[0])
	fmt.Printf("    Image   : %s\n",   s.Image)
	fmt.Printf("    Status  : %s\n",   s.Status)
	fmt.Printf("    State   : %s\n",   s.State)
	fmt.Printf("    Command : %s\n\n", s.Command)
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
	// https://docs.docker.com/reference/api/engine/version/v1.39/
	if len(os.Args) < 2{
		fmt.Printf("No private key path provided.\n")
		fmt.Printf("Usage:\n%s [path_to_private_key]\n", os.Args[0])
		os.Exit(1)
	}
	privateKeyPath := os.Args[1]
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
		fmt.Printf("Dock %s@%s\n", dock.Username, dock.Host)
		client, err := dock.Connect(privateKeyPath)
		if err != nil{
			fmt.Printf("Could not connect to dock %s, %s\n", dock.Host, err)
			continue
		}
		statuses, err := dock.GetStatus(client, remoteSocketPath)
		if err != nil{
			fmt.Printf("Could not GetStatus of Dock %s, %s\n", dock.Host, err)
			continue
		}
		for _, s := range statuses{
			s.Display()
			// dock.GetLogs(client, remoteSocketPath, s.Id)
		}
	}
}
