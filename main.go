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

	err := cmd.Start()
	errChan := make(chan error)

	if err != nil {
		log.Printf("command %v failed.\nstdout : %v\nstderr :%v", cmd.Stdin, stdout.String(), stderr.String())
		if exitError, ok := err.(*exec.ExitError); ok {
			exitStatus = int32(exitError.ExitCode())
		}
	}

	go func() {
		errChan <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return &bashpb.CommandResponse{
			Stdout:     "Request was cancelled",
			ExitStatus: 1,
		}, nil
	case <-errChan:
		break
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

func (s *BashServiceServer) ExecuteAndStream(req *bashpb.CommandRequest, stream bashpb.BashService_ExecuteAndStreamServer) error {
	// Put the command in a string slice
	// Declare the current working directory
	dir := req.Cwd

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("cmd", "/c", req.Command)

	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	var exitStatus int32

	err := cmd.Start()
	if err != nil {
		log.Printf("command %v failed.\nstdout : %v\nstderr :%v", cmd.Stdin, stdout.String(), stderr.String())
		if exitError, ok := err.(*exec.ExitError); ok {
			exitStatus = int32(exitError.ExitCode())
		}
	}

	errChan := make(chan error)

	go func() {
		errChan <- cmd.Wait()
	}()

	for {
		select {
		case err = <-errChan:
			if err != nil {
				log.Printf("command %v failed.\nstdout : %v\nstderr :%v", cmd.Stdin, stdout.String(), stderr.String())
				if exitError, ok := err.(*exec.ExitError); ok {
					exitStatus = int32(exitError.ExitCode())
				}
			}
			_ = stream.Send(&bashpb.CommandResponse{
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitStatus: exitStatus,
			})
			return nil
		default:
			err = stream.Send(&bashpb.CommandResponse{
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitStatus: exitStatus,
			})
			if err != nil {
				fmt.Printf("cannot send to stream.\n")
				_ = cmd.Process.Kill()
				return err
			}
			fmt.Printf("Sent response\n")
			time.Sleep(500 * time.Millisecond)
		}
	}

}
