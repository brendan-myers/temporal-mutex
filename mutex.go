package temporalmutex

import (
	"context"
	"math/rand"
	"time"

	"go.temporal.io/api/command/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	TIMEOUT             = 60 * time.Second
	ShortTimeout        = 30 * time.Second
	StartToCloseTimeout = 30 * time.Second
	TaskQueue           = "default"
	Namespace           = "default"
	WorkflowType        = "lock"
	WorkflowId          = "lock"
	ActivityType        = "lock"
	ActivityId          = "lock"
)

type Mutex struct {
	conn      *grpc.ClientConn
	client    *workflowservice.WorkflowServiceClient
	taskToken []byte
}

func NewMutex(ctx context.Context, target string) (*Mutex, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &Mutex{}, err
	}

	client := workflowservice.NewWorkflowServiceClient(conn)

	m := &Mutex{conn: conn, client: &client}

	err = m.createLockWorkflow(ctx)
	if err != nil {
		return &Mutex{}, err
	}

	err = m.createLockActivity(ctx)
	if err != nil {
		return &Mutex{}, err
	}

	return m, nil
}

func (m *Mutex) Close() {
	if m.conn != nil {
		_ = m.conn.Close()
	}
}

func (m *Mutex) Lock(ctx context.Context) error {
	if m.taskToken != nil {
		// TODO - Lock() may have been called twice
		return nil
	}

	for {
		lctx, cancel := context.WithTimeout(ctx, TIMEOUT)

		// try to obtain the lock
		res, err := (*m.client).PollActivityTaskQueue(lctx, &workflowservice.PollActivityTaskQueueRequest{
			Namespace: Namespace,
			TaskQueue: &taskqueue.TaskQueue{
				Name: TaskQueue,
			},
		})
		if err != nil {
			cancel()
			return err
		}

		// if the lock has been obtained, return
		if res.TaskToken != nil {
			m.taskToken = res.TaskToken
			cancel()
			return nil
		}

		// If the lock wasn't obtained, randomly check if workflow and activity
		// tasks need to be created (as they may have been lost due to process exit/crash/etc)
		if rand.Intn(100) >= 50 {
			_ = m.createLockActivity(ctx)
		}
		cancel()
	}
}

func (m *Mutex) Unlock(ctx context.Context) error {
	// TODO should error if lock activity timed out

	_, err := (*m.client).RespondActivityTaskCompleted(ctx, &workflowservice.RespondActivityTaskCompletedRequest{
		TaskToken: m.taskToken,
		Namespace: Namespace,
	})

	if err == nil {
		m.taskToken = nil
	}

	return m.createLockActivity(ctx)
}

func (m *Mutex) createLockWorkflow(ctx context.Context) error {
	_, err := (*m.client).SignalWithStartWorkflowExecution(ctx, &workflowservice.SignalWithStartWorkflowExecutionRequest{
		Namespace:  Namespace,
		WorkflowId: WorkflowId,
		WorkflowType: &common.WorkflowType{
			Name: WorkflowType,
		},
		TaskQueue: &taskqueue.TaskQueue{
			Name: TaskQueue,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
		SignalName: "start",
	})

	return err
}

func (m *Mutex) createLockActivity(ctx context.Context) error {
	lctx, cancel := context.WithTimeout(ctx, ShortTimeout)
	defer cancel()

	wRes, err := (*m.client).PollWorkflowTaskQueue(lctx, &workflowservice.PollWorkflowTaskQueueRequest{
		Namespace: Namespace,
		TaskQueue: &taskqueue.TaskQueue{
			Name: TaskQueue,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
	})
	if err != nil || wRes.TaskToken == nil {
		return nil
	}

	mctx, cancel := context.WithTimeout(ctx, ShortTimeout)
	defer cancel()

	_, err = (*m.client).RespondWorkflowTaskCompleted(mctx, &workflowservice.RespondWorkflowTaskCompletedRequest{
		TaskToken: wRes.TaskToken,
		Namespace: Namespace,
		Commands: []*command.Command{
			{
				CommandType: enums.COMMAND_TYPE_SCHEDULE_ACTIVITY_TASK,
				Attributes: &command.Command_ScheduleActivityTaskCommandAttributes{
					ScheduleActivityTaskCommandAttributes: &command.ScheduleActivityTaskCommandAttributes{
						ActivityId: ActivityId,
						ActivityType: &common.ActivityType{
							Name: ActivityType,
						},
						TaskQueue: &taskqueue.TaskQueue{
							Name: TaskQueue,
							Kind: enums.TASK_QUEUE_KIND_NORMAL,
						},
						StartToCloseTimeout: durationpb.New(StartToCloseTimeout),
					},
				},
			},
		},
	})

	return err
}
