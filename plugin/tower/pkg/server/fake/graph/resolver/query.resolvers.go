package resolver

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/smartxworks/lynx/plugin/tower/pkg/schema"
	"github.com/smartxworks/lynx/plugin/tower/pkg/server/fake/graph/generated"
	"github.com/smartxworks/lynx/plugin/tower/pkg/server/fake/graph/model"
)

func (r *mutationResolver) Login(ctx context.Context, data model.LoginInput) (*model.Login, error) {
	obj, exists := r.Resolver.TrackerFactory().User().Get(data.Username)
	if !exists {
		return nil, fmt.Errorf("user %s not found", data.Username)
	}

	user := obj.(*model.User)

	if user.Source != data.Source {
		return nil, fmt.Errorf("user %s not found", data.Username)
	}
	if user.Password != data.Password {
		return nil, fmt.Errorf("the specified password for user %s is not correct", user.Name)
	}

	return &model.Login{Token: user.Token}, nil
}

func (r *queryResolver) Vms(ctx context.Context) ([]schema.VM, error) {
	vmList := r.TrackerFactory().VM().List()
	vms := make([]schema.VM, 0, len(vmList))
	for _, vm := range vmList {
		vms = append(vms, *vm.(*schema.VM))
	}
	return vms, nil
}

func (r *queryResolver) Labels(ctx context.Context) ([]schema.Label, error) {
	labelList := r.TrackerFactory().Label().List()
	labels := make([]schema.Label, 0, len(labelList))
	for _, label := range labelList {
		labels = append(labels, *label.(*schema.Label))
	}
	return labels, nil
}

func (r *subscriptionResolver) VM(ctx context.Context) (<-chan *model.VMEvent, error) {
	var vmEventCh = make(chan *model.VMEvent, 100)

	go func() {
		eventCh, stopWatch := r.TrackerFactory().VM().Watch()
		defer stopWatch()
		defer close(vmEventCh)

		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				vmEventCh <- &model.VMEvent{
					Mutation: event.Type,
					Node:     event.Object.(*schema.VM),
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return vmEventCh, nil
}

func (r *subscriptionResolver) Label(ctx context.Context) (<-chan *model.LabelEvent, error) {
	var labelEventCh = make(chan *model.LabelEvent, 100)

	go func() {
		eventCh, stopWatch := r.TrackerFactory().Label().Watch()
		defer stopWatch()

		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				labelEventCh <- &model.LabelEvent{
					Mutation: event.Type,
					Node:     event.Object.(*schema.Label),
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return labelEventCh, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
