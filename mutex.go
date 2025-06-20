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
	TIMEOUT                time.Duration = 60 * time.Second
	SHORT_TIMEOUT          time.Duration = 30 * time.Second
	START_TO_CLOSE_TIMEOUT time.Duration = 30 * time.Second
	TASK_QUEUE             string        = "default"
	NAMESPACE              string        = "default"
	WORKFLOW_TYPE          string        = "lock"
	WORKFLOW_ID            string        = "lock"
	ACTIVITY_TYPE          string        = "lock"
	ACTIVITY_ID            string        = "lock"
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

	//err
	_ = m.createLockActivity(ctx)
	//if err != nil {
	//	return &Mutex{}, err
	//}

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
			Namespace: NAMESPACE,
			TaskQueue: &taskqueue.TaskQueue{
				Name: TASK_QUEUE,
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
		Namespace: NAMESPACE,
	})

	if err == nil {
		m.taskToken = nil
	}

	return m.createLockActivity(ctx)
}

func (m *Mutex) createLockWorkflow(ctx context.Context) error {
	_, err := (*m.client).SignalWithStartWorkflowExecution(ctx, &workflowservice.SignalWithStartWorkflowExecutionRequest{
		Namespace:  NAMESPACE,
		WorkflowId: WORKFLOW_ID,
		WorkflowType: &common.WorkflowType{
			Name: WORKFLOW_TYPE,
		},
		TaskQueue: &taskqueue.TaskQueue{
			Name: TASK_QUEUE,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
		SignalName: "start",
	})

	return err
}

func (m *Mutex) createLockActivity(ctx context.Context) error {
	lctx, cancel := context.WithTimeout(ctx, SHORT_TIMEOUT)
	defer cancel()

	wRes, err := (*m.client).PollWorkflowTaskQueue(lctx, &workflowservice.PollWorkflowTaskQueueRequest{
		Namespace: NAMESPACE,
		TaskQueue: &taskqueue.TaskQueue{
			Name: TASK_QUEUE,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
	})
	if err != nil || wRes.TaskToken == nil {
		return nil
	}

	mctx, cancel := context.WithTimeout(ctx, SHORT_TIMEOUT)
	defer cancel()

	_, err = (*m.client).RespondWorkflowTaskCompleted(mctx, &workflowservice.RespondWorkflowTaskCompletedRequest{
		TaskToken: wRes.TaskToken,
		Namespace: NAMESPACE,
		Commands: []*command.Command{
			{
				CommandType: enums.COMMAND_TYPE_SCHEDULE_ACTIVITY_TASK,
				Attributes: &command.Command_ScheduleActivityTaskCommandAttributes{
					ScheduleActivityTaskCommandAttributes: &command.ScheduleActivityTaskCommandAttributes{
						ActivityId: ACTIVITY_ID,
						ActivityType: &common.ActivityType{
							Name: ACTIVITY_TYPE,
						},
						TaskQueue: &taskqueue.TaskQueue{
							Name: TASK_QUEUE,
							Kind: enums.TASK_QUEUE_KIND_NORMAL,
						},
						StartToCloseTimeout: durationpb.New(START_TO_CLOSE_TIMEOUT),
					},
				},
			},
		},
	})

	return err
}
