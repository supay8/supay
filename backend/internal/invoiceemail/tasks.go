package invoiceemail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CloudTasksConfig struct {
	ProjectID           string
	Location            string
	Queue               string
	WorkerURL           string
	ServiceAccountEmail string
	Audience            string
}

func (c CloudTasksConfig) Validate() error {
	if strings.TrimSpace(c.ProjectID) == "" || strings.TrimSpace(c.Location) == "" || strings.TrimSpace(c.Queue) == "" {
		return errors.New("Cloud Tasks requiere project, location y queue")
	}
	if !strings.HasPrefix(c.WorkerURL, "https://") {
		return errors.New("EMAIL_WORKER_URL debe ser HTTPS")
	}
	if strings.TrimSpace(c.ServiceAccountEmail) == "" {
		return errors.New("CLOUD_TASKS_SERVICE_ACCOUNT_EMAIL es obligatorio")
	}
	return nil
}

type TaskPublisher interface {
	Publish(ctx context.Context, notificationID string) (string, error)
}

type CloudTasksPublisher struct {
	client *cloudtasks.Client
	cfg    CloudTasksConfig
}

func NewCloudTasksPublisher(client *cloudtasks.Client, cfg CloudTasksConfig) (*CloudTasksPublisher, error) {
	if client == nil {
		return nil, errors.New("cliente Cloud Tasks no configurado")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Audience) == "" {
		cfg.Audience = strings.TrimRight(cfg.WorkerURL, "/")
	}
	return &CloudTasksPublisher{client: client, cfg: cfg}, nil
}

func (p *CloudTasksPublisher) Publish(ctx context.Context, notificationID string) (string, error) {
	if _, err := uuid.Parse(notificationID); err != nil {
		return "", fmt.Errorf("notificación inválida: %w", err)
	}
	body, err := json.Marshal(map[string]string{"notificacion_id": notificationID})
	if err != nil {
		return "", err
	}
	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s", p.cfg.ProjectID, p.cfg.Location, p.cfg.Queue)
	taskName := parent + "/tasks/invoice-email-" + strings.ReplaceAll(notificationID, "-", "")
	workerEndpoint := strings.TrimRight(p.cfg.WorkerURL, "/") + "/internal/tasks/send-invoice-email"
	task := &cloudtaskspb.Task{
		Name: taskName,
		MessageType: &cloudtaskspb.Task_HttpRequest{HttpRequest: &cloudtaskspb.HttpRequest{
			HttpMethod: cloudtaskspb.HttpMethod_POST,
			Url:        workerEndpoint,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       body,
			AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{OidcToken: &cloudtaskspb.OidcToken{
				ServiceAccountEmail: p.cfg.ServiceAccountEmail,
				Audience:            p.cfg.Audience,
			}},
		}},
	}
	created, err := p.client.CreateTask(ctx, &cloudtaskspb.CreateTaskRequest{Parent: parent, Task: task})
	if err != nil {
		// A deterministic task name closes the "response lost after create" gap.
		// ALREADY_EXISTS means Cloud Tasks already owns delivery.
		if status.Code(err) == codes.AlreadyExists {
			return taskName, nil
		}
		return "", err
	}
	return created.GetName(), nil
}

type Dispatcher struct {
	repo        domain.InvoiceEmailNotificationRepository
	publisher   TaskPublisher
	owner       string
	batchSize   int
	lockTimeout time.Duration
	now         func() time.Time
}

func NewDispatcher(repo domain.InvoiceEmailNotificationRepository, publisher TaskPublisher, batchSize int, lockTimeout time.Duration) *Dispatcher {
	if batchSize <= 0 {
		batchSize = 100
	}
	if lockTimeout <= 0 {
		lockTimeout = 30 * time.Second
	}
	return &Dispatcher{repo: repo, publisher: publisher, owner: "email-dispatch-" + uuid.NewString(), batchSize: batchSize, lockTimeout: lockTimeout, now: func() time.Time { return time.Now().UTC() }}
}

func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	_, err := d.DispatchBatch(ctx)
	return err
}

func (d *Dispatcher) DispatchBatch(ctx context.Context) (int, error) {
	if d == nil || d.repo == nil || d.publisher == nil {
		return 0, errors.New("dispatcher de email no configurado")
	}
	now := d.now()
	notifications, err := d.repo.ClaimPending(ctx, d.owner, d.batchSize, now, d.lockTimeout)
	if err != nil {
		return 0, fmt.Errorf("reclamar notificaciones: %w", err)
	}
	var errs []error
	for _, notification := range notifications {
		taskName, publishErr := d.publisher.Publish(ctx, notification.ID)
		if publishErr != nil {
			next := now.Add(emailRetryDelay(notification.PublishAttempts))
			if markErr := d.repo.MarkPublishFailed(ctx, notification.ID, d.owner, publishErr.Error(), next); markErr != nil {
				errs = append(errs, errors.Join(publishErr, markErr))
			} else {
				errs = append(errs, publishErr)
			}
			continue
		}
		if err := d.repo.MarkEnqueued(ctx, notification.ID, d.owner, taskName, d.now()); err != nil {
			errs = append(errs, err)
		}
	}
	return len(notifications), errors.Join(errs...)
}

func emailRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 8)
	return min(time.Second*time.Duration(1<<shift), 5*time.Minute)
}
