/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sequence

import (
	"context"

	"k8s.io/client-go/tools/cache"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/flows/v1/sequence"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	sequencereconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/sequence"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	sequenceInformer := sequence.Get(ctx)
	subscriptionInformer := subscription.Get(ctx)
	eventPolicyInformer := eventpolicy.Get(ctx)

	r := &Reconciler{
		sequenceLister:     sequenceInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		dynamicClientSet:   dynamicclient.Get(ctx),
		eventingClientSet:  eventingclient.Get(ctx),
		eventPolicyLister:  eventPolicyInformer.Lister(),
	}
	impl := sequencereconciler.NewImpl(ctx, r)

	r.channelableTracker = duck.NewListableTrackerFromTracker(ctx, channelable.Get, impl.Tracker)
	sequenceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Register handler for Subscriptions that are owned by Sequence, so that
	// we get notified if they change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.Sequence{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	sequenceGK := flowsv1.SchemeGroupVersion.WithKind("Sequence").GroupKind()

	// Enqueue the Sequence, if we have an EventPolicy which was referencing
	// or got updated and now is referencing the Sequence
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(
		sequenceInformer.Informer().GetIndexer(),
		sequenceGK,
		impl.EnqueueKey,
	))

	return impl
}
