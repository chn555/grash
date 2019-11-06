package main

import (
	"bytes"
	"context"
	"fmt"
	bashpb "github.com/chn555/grash/bash"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

type BashServiceServer struct{}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("Starting server on port :50051")

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Unable to listen on port :50051: %v", err)
	}

	var opts []grpc.ServerOption

	s := grpc.NewServer(opts...)

	srv := &BashServiceServer{}

	bashpb.RegisterBashServiceServer(s, srv)

	go func() {
		if err := s.Serve(listener); err != nil {
			log.Fatalf("Failed to server : %v", err)
		}
	}()
	fmt.Println("Server successfully started on port :50051")

	// Right way to stop the server using a SHUTDOWN HOOK
	// Create a channel to receive OS signals
	c := make(chan os.Signal)

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	// Ignore other incoming signals
	signal.Notify(c, os.Interrupt)

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-c
	fmt.Println("\nStopping the server")
	s.Stop()
	_ = listener.Close()
	fmt.Println("Done.")
}

func (s *BashServiceServer) Execute(ctx context.Context, req *bashpb.CommandRequest) (*bashpb.CommandResponse, error) {
	//if runtime.GOOS == "windows" {
	//	return nil, fmt.Errorf("an't Execute this on a windows machine")
	//
	//}
	//out, err := exec.Command("cmd", "/c", req.Command).Output()

	return RunCommand(req)
}

func RunCommand(req *bashpb.CommandRequest) (*bashpb.CommandResponse, error) {
	// Put the command in a string slice
	// Declare the current working directory
	dir := req.Cwd

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("cmd", "/c", req.Command)

	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var exitStatus int32 = 0

	err := cmd.Run()
	if err != nil {
		log.Printf("command %v failed.\nstdout : %v\nstderr :%v", cmd.Stdin, stdout.String(), stderr.String())
		if exitError, ok := err.(*exec.ExitError); ok {
			exitStatus = int32(exitError.ExitCode())
		}
	}

	resp := &bashpb.CommandResponse{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitStatus: exitStatus,
	}
	if err != nil {
		resp.ExitStatus = 1
	}

	return resp, nil
}

func (s *BashServiceServer) ExecuteAndPoll(req *bashpb.CommandRequest, stream bashpb.BashService_ExecuteAndPollServer) error {
	// Put the command in a string slice
	// Declare the current working directory
	for {
		resp, err := RunCommand(req)
		if err != nil {
			return err
		}
		if err = stream.Send(resp); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
}
