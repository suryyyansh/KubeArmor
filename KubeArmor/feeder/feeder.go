package feeder

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	kg "github.com/accuknox/KubeArmor/KubeArmor/log"
	tp "github.com/accuknox/KubeArmor/KubeArmor/types"

	pb "github.com/accuknox/KubeArmor/KubeArmor/feeder/protobuf"
	"google.golang.org/grpc"
)

// Feeder Structure
type Feeder struct {
	// server
	server string

	// log type
	logType string

	// connection
	conn *grpc.ClientConn

	// client
	client pb.LogMessageClient

	// audit log stream
	auditLogStream pb.LogMessage_AuditLogsClient

	// system log stream
	systemLogStream pb.LogMessage_SystemLogsClient
}

// NewFeeder Function
func NewFeeder(server, logType string) *Feeder {
	fd := &Feeder{}

	fd.server = server
	fd.logType = logType

	for {
		_, ok := fd.DoHealthCheck()
		if ok {
			break
		}
		time.Sleep(time.Second * 1)
	}

	conn, err := grpc.Dial(fd.server, grpc.WithInsecure())
	if err != nil {
		kg.Err(err.Error())
		return nil
	}

	fd.conn = conn

	if logType == "AuditLog" {
		fd.client = pb.NewLogMessageClient(fd.conn)

		stream, err := fd.client.AuditLogs(context.Background())
		if err != nil {
			kg.Err(err.Error())
			return nil
		}

		fd.auditLogStream = stream
	} else if logType == "SystemLog" {
		fd.client = pb.NewLogMessageClient(fd.conn)

		stream, err := fd.client.SystemLogs(context.Background())
		if err != nil {
			kg.Err(err.Error())
			return nil
		}

		fd.systemLogStream = stream
	} else {
		kg.Printf("Not supported type (%s)", logType)
		fd.conn.Close()
		fd.conn = nil
		return nil
	}

	return fd
}

// DestroyFeeder Function
func (fd *Feeder) DestroyFeeder() {
	fd.conn.Close()
}

// DoHealthCheck Function
func (fd *Feeder) DoHealthCheck() (string, bool) {
	// connect to server
	conn, err := grpc.Dial(fd.server, grpc.WithInsecure())
	if err != nil {
		kg.Err(err.Error())
		return fmt.Sprintf("Failed to connect the server (%s)", fd.server), false
	}
	defer conn.Close()

	// set client
	client := pb.NewLogMessageClient(conn)

	// generate nonce
	rand := rand.Int31()

	// send a nonce
	nonce := pb.NonceMessage{Nonce: rand}
	res, err := client.HealthCheck(context.Background(), &nonce)
	if err != nil {
		return err.Error(), false
	}

	// check nonces
	if rand != res.Retval {
		return "Nonces are different", false
	}

	return "success", true
}

// SendAuditLog Function
func (fd *Feeder) SendAuditLog(auditLog tp.AuditLog) {
	if fd.conn == nil {
		kg.Print("gRPC is not set")
		return
	} else if fd.logType == "SystemLog" {
		kg.Print("gRPC is set for system logs (not audit logs)")
		return
	}

	log := pb.AuditLog{}

	log.UpdatedTime = auditLog.UpdatedTime

	log.HostName = auditLog.HostName

	log.ContainerID = auditLog.ContainerID
	log.ContainerName = auditLog.ContainerName

	log.HostPID = auditLog.HostPID
	log.Source = auditLog.Source
	log.Operation = auditLog.Operation
	log.Resource = auditLog.Resource
	log.Action = auditLog.Action

	log.RawData = auditLog.RawData

	fd.auditLogStream.Send(&log)
}

// SendSystemLog Function
func (fd *Feeder) SendSystemLog(systemLog tp.SystemLog) {
	if fd.conn == nil {
		kg.Print("gRPC is not set")
		return
	} else if fd.logType == "AuditLog" {
		kg.Print("gRPC is set for audit logs (not system logs)")
		return
	}

	log := pb.SystemLog{}

	log.UpdatedTime = systemLog.UpdatedTime

	log.HostName = systemLog.HostName

	log.ContainerID = systemLog.ContainerID
	log.ContainerName = systemLog.ContainerName

	log.HostPID = systemLog.HostPID
	log.PPID = systemLog.PPID
	log.PID = systemLog.PID
	log.TID = systemLog.TID
	log.UID = systemLog.UID
	log.Comm = systemLog.Comm

	log.Syscall = systemLog.Syscall
	log.Argnum = systemLog.Argnum
	log.Retval = systemLog.Retval

	log.Data = systemLog.Data

	if len(systemLog.ErrorMessage) > 0 {
		log.ErrorMessage = systemLog.ErrorMessage
	}

	fd.systemLogStream.Send(&log)
}
