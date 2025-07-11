//go:build go1.18
// +build go1.18

// Code generated by Microsoft (R) AutoRest Code Generator (autorest: 3.10.8, generator: @autorest/go@4.0.0-preview.63)
// Changes may cause incorrect behavior and will be lost if the code is regenerated.
// Code generated by @autorest/go. DO NOT EDIT.

package fake

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake/server"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/Azure/ARO-HCP/internal/api/v20240610preview/generated"
)

// NodePoolsServer is a fake server for instances of the generated.NodePoolsClient type.
type NodePoolsServer struct {
	// BeginCreateOrUpdate is the fake for method NodePoolsClient.BeginCreateOrUpdate
	// HTTP status codes to indicate success: http.StatusOK, http.StatusCreated
	BeginCreateOrUpdate func(ctx context.Context, resourceGroupName string, hcpOpenShiftClusterName string, nodePoolName string, resource generated.NodePool, options *generated.NodePoolsClientBeginCreateOrUpdateOptions) (resp azfake.PollerResponder[generated.NodePoolsClientCreateOrUpdateResponse], errResp azfake.ErrorResponder)

	// BeginDelete is the fake for method NodePoolsClient.BeginDelete
	// HTTP status codes to indicate success: http.StatusAccepted, http.StatusNoContent
	BeginDelete func(ctx context.Context, resourceGroupName string, hcpOpenShiftClusterName string, nodePoolName string, options *generated.NodePoolsClientBeginDeleteOptions) (resp azfake.PollerResponder[generated.NodePoolsClientDeleteResponse], errResp azfake.ErrorResponder)

	// Get is the fake for method NodePoolsClient.Get
	// HTTP status codes to indicate success: http.StatusOK
	Get func(ctx context.Context, resourceGroupName string, hcpOpenShiftClusterName string, nodePoolName string, options *generated.NodePoolsClientGetOptions) (resp azfake.Responder[generated.NodePoolsClientGetResponse], errResp azfake.ErrorResponder)

	// NewListByParentPager is the fake for method NodePoolsClient.NewListByParentPager
	// HTTP status codes to indicate success: http.StatusOK
	NewListByParentPager func(resourceGroupName string, hcpOpenShiftClusterName string, options *generated.NodePoolsClientListByParentOptions) (resp azfake.PagerResponder[generated.NodePoolsClientListByParentResponse])

	// BeginUpdate is the fake for method NodePoolsClient.BeginUpdate
	// HTTP status codes to indicate success: http.StatusOK, http.StatusAccepted
	BeginUpdate func(ctx context.Context, resourceGroupName string, hcpOpenShiftClusterName string, nodePoolName string, properties generated.NodePoolUpdate, options *generated.NodePoolsClientBeginUpdateOptions) (resp azfake.PollerResponder[generated.NodePoolsClientUpdateResponse], errResp azfake.ErrorResponder)
}

// NewNodePoolsServerTransport creates a new instance of NodePoolsServerTransport with the provided implementation.
// The returned NodePoolsServerTransport instance is connected to an instance of generated.NodePoolsClient via the
// azcore.ClientOptions.Transporter field in the client's constructor parameters.
func NewNodePoolsServerTransport(srv *NodePoolsServer) *NodePoolsServerTransport {
	return &NodePoolsServerTransport{
		srv:                  srv,
		beginCreateOrUpdate:  newTracker[azfake.PollerResponder[generated.NodePoolsClientCreateOrUpdateResponse]](),
		beginDelete:          newTracker[azfake.PollerResponder[generated.NodePoolsClientDeleteResponse]](),
		newListByParentPager: newTracker[azfake.PagerResponder[generated.NodePoolsClientListByParentResponse]](),
		beginUpdate:          newTracker[azfake.PollerResponder[generated.NodePoolsClientUpdateResponse]](),
	}
}

// NodePoolsServerTransport connects instances of generated.NodePoolsClient to instances of NodePoolsServer.
// Don't use this type directly, use NewNodePoolsServerTransport instead.
type NodePoolsServerTransport struct {
	srv                  *NodePoolsServer
	beginCreateOrUpdate  *tracker[azfake.PollerResponder[generated.NodePoolsClientCreateOrUpdateResponse]]
	beginDelete          *tracker[azfake.PollerResponder[generated.NodePoolsClientDeleteResponse]]
	newListByParentPager *tracker[azfake.PagerResponder[generated.NodePoolsClientListByParentResponse]]
	beginUpdate          *tracker[azfake.PollerResponder[generated.NodePoolsClientUpdateResponse]]
}

// Do implements the policy.Transporter interface for NodePoolsServerTransport.
func (n *NodePoolsServerTransport) Do(req *http.Request) (*http.Response, error) {
	rawMethod := req.Context().Value(runtime.CtxAPINameKey{})
	method, ok := rawMethod.(string)
	if !ok {
		return nil, nonRetriableError{errors.New("unable to dispatch request, missing value for CtxAPINameKey")}
	}

	var resp *http.Response
	var err error

	switch method {
	case "NodePoolsClient.BeginCreateOrUpdate":
		resp, err = n.dispatchBeginCreateOrUpdate(req)
	case "NodePoolsClient.BeginDelete":
		resp, err = n.dispatchBeginDelete(req)
	case "NodePoolsClient.Get":
		resp, err = n.dispatchGet(req)
	case "NodePoolsClient.NewListByParentPager":
		resp, err = n.dispatchNewListByParentPager(req)
	case "NodePoolsClient.BeginUpdate":
		resp, err = n.dispatchBeginUpdate(req)
	default:
		err = fmt.Errorf("unhandled API %s", method)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (n *NodePoolsServerTransport) dispatchBeginCreateOrUpdate(req *http.Request) (*http.Response, error) {
	if n.srv.BeginCreateOrUpdate == nil {
		return nil, &nonRetriableError{errors.New("fake for method BeginCreateOrUpdate not implemented")}
	}
	beginCreateOrUpdate := n.beginCreateOrUpdate.get(req)
	if beginCreateOrUpdate == nil {
		const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/resourceGroups/(?P<resourceGroupName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/providers/Microsoft\.RedHatOpenShift/hcpOpenShiftClusters/(?P<hcpOpenShiftClusterName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/nodePools/(?P<nodePoolName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)`
		regex := regexp.MustCompile(regexStr)
		matches := regex.FindStringSubmatch(req.URL.EscapedPath())
		if matches == nil || len(matches) < 4 {
			return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
		}
		body, err := server.UnmarshalRequestAsJSON[generated.NodePool](req)
		if err != nil {
			return nil, err
		}
		resourceGroupNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("resourceGroupName")])
		if err != nil {
			return nil, err
		}
		hcpOpenShiftClusterNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("hcpOpenShiftClusterName")])
		if err != nil {
			return nil, err
		}
		nodePoolNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("nodePoolName")])
		if err != nil {
			return nil, err
		}
		respr, errRespr := n.srv.BeginCreateOrUpdate(req.Context(), resourceGroupNameParam, hcpOpenShiftClusterNameParam, nodePoolNameParam, body, nil)
		if respErr := server.GetError(errRespr, req); respErr != nil {
			return nil, respErr
		}
		beginCreateOrUpdate = &respr
		n.beginCreateOrUpdate.add(req, beginCreateOrUpdate)
	}

	resp, err := server.PollerResponderNext(beginCreateOrUpdate, req)
	if err != nil {
		return nil, err
	}

	if !contains([]int{http.StatusOK, http.StatusCreated}, resp.StatusCode) {
		n.beginCreateOrUpdate.remove(req)
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusOK, http.StatusCreated", resp.StatusCode)}
	}
	if !server.PollerResponderMore(beginCreateOrUpdate) {
		n.beginCreateOrUpdate.remove(req)
	}

	return resp, nil
}

func (n *NodePoolsServerTransport) dispatchBeginDelete(req *http.Request) (*http.Response, error) {
	if n.srv.BeginDelete == nil {
		return nil, &nonRetriableError{errors.New("fake for method BeginDelete not implemented")}
	}
	beginDelete := n.beginDelete.get(req)
	if beginDelete == nil {
		const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/resourceGroups/(?P<resourceGroupName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/providers/Microsoft\.RedHatOpenShift/hcpOpenShiftClusters/(?P<hcpOpenShiftClusterName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/nodePools/(?P<nodePoolName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)`
		regex := regexp.MustCompile(regexStr)
		matches := regex.FindStringSubmatch(req.URL.EscapedPath())
		if matches == nil || len(matches) < 4 {
			return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
		}
		resourceGroupNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("resourceGroupName")])
		if err != nil {
			return nil, err
		}
		hcpOpenShiftClusterNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("hcpOpenShiftClusterName")])
		if err != nil {
			return nil, err
		}
		nodePoolNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("nodePoolName")])
		if err != nil {
			return nil, err
		}
		respr, errRespr := n.srv.BeginDelete(req.Context(), resourceGroupNameParam, hcpOpenShiftClusterNameParam, nodePoolNameParam, nil)
		if respErr := server.GetError(errRespr, req); respErr != nil {
			return nil, respErr
		}
		beginDelete = &respr
		n.beginDelete.add(req, beginDelete)
	}

	resp, err := server.PollerResponderNext(beginDelete, req)
	if err != nil {
		return nil, err
	}

	if !contains([]int{http.StatusAccepted, http.StatusNoContent}, resp.StatusCode) {
		n.beginDelete.remove(req)
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusAccepted, http.StatusNoContent", resp.StatusCode)}
	}
	if !server.PollerResponderMore(beginDelete) {
		n.beginDelete.remove(req)
	}

	return resp, nil
}

func (n *NodePoolsServerTransport) dispatchGet(req *http.Request) (*http.Response, error) {
	if n.srv.Get == nil {
		return nil, &nonRetriableError{errors.New("fake for method Get not implemented")}
	}
	const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/resourceGroups/(?P<resourceGroupName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/providers/Microsoft\.RedHatOpenShift/hcpOpenShiftClusters/(?P<hcpOpenShiftClusterName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/nodePools/(?P<nodePoolName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)`
	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(req.URL.EscapedPath())
	if matches == nil || len(matches) < 4 {
		return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
	}
	resourceGroupNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("resourceGroupName")])
	if err != nil {
		return nil, err
	}
	hcpOpenShiftClusterNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("hcpOpenShiftClusterName")])
	if err != nil {
		return nil, err
	}
	nodePoolNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("nodePoolName")])
	if err != nil {
		return nil, err
	}
	respr, errRespr := n.srv.Get(req.Context(), resourceGroupNameParam, hcpOpenShiftClusterNameParam, nodePoolNameParam, nil)
	if respErr := server.GetError(errRespr, req); respErr != nil {
		return nil, respErr
	}
	respContent := server.GetResponseContent(respr)
	if !contains([]int{http.StatusOK}, respContent.HTTPStatus) {
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusOK", respContent.HTTPStatus)}
	}
	resp, err := server.MarshalResponseAsJSON(respContent, server.GetResponse(respr).NodePool, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (n *NodePoolsServerTransport) dispatchNewListByParentPager(req *http.Request) (*http.Response, error) {
	if n.srv.NewListByParentPager == nil {
		return nil, &nonRetriableError{errors.New("fake for method NewListByParentPager not implemented")}
	}
	newListByParentPager := n.newListByParentPager.get(req)
	if newListByParentPager == nil {
		const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/resourceGroups/(?P<resourceGroupName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/providers/Microsoft\.RedHatOpenShift/hcpOpenShiftClusters/(?P<hcpOpenShiftClusterName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/nodePools`
		regex := regexp.MustCompile(regexStr)
		matches := regex.FindStringSubmatch(req.URL.EscapedPath())
		if matches == nil || len(matches) < 3 {
			return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
		}
		resourceGroupNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("resourceGroupName")])
		if err != nil {
			return nil, err
		}
		hcpOpenShiftClusterNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("hcpOpenShiftClusterName")])
		if err != nil {
			return nil, err
		}
		resp := n.srv.NewListByParentPager(resourceGroupNameParam, hcpOpenShiftClusterNameParam, nil)
		newListByParentPager = &resp
		n.newListByParentPager.add(req, newListByParentPager)
		server.PagerResponderInjectNextLinks(newListByParentPager, req, func(page *generated.NodePoolsClientListByParentResponse, createLink func() string) {
			page.NextLink = to.Ptr(createLink())
		})
	}
	resp, err := server.PagerResponderNext(newListByParentPager, req)
	if err != nil {
		return nil, err
	}
	if !contains([]int{http.StatusOK}, resp.StatusCode) {
		n.newListByParentPager.remove(req)
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusOK", resp.StatusCode)}
	}
	if !server.PagerResponderMore(newListByParentPager) {
		n.newListByParentPager.remove(req)
	}
	return resp, nil
}

func (n *NodePoolsServerTransport) dispatchBeginUpdate(req *http.Request) (*http.Response, error) {
	if n.srv.BeginUpdate == nil {
		return nil, &nonRetriableError{errors.New("fake for method BeginUpdate not implemented")}
	}
	beginUpdate := n.beginUpdate.get(req)
	if beginUpdate == nil {
		const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/resourceGroups/(?P<resourceGroupName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/providers/Microsoft\.RedHatOpenShift/hcpOpenShiftClusters/(?P<hcpOpenShiftClusterName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/nodePools/(?P<nodePoolName>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)`
		regex := regexp.MustCompile(regexStr)
		matches := regex.FindStringSubmatch(req.URL.EscapedPath())
		if matches == nil || len(matches) < 4 {
			return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
		}
		body, err := server.UnmarshalRequestAsJSON[generated.NodePoolUpdate](req)
		if err != nil {
			return nil, err
		}
		resourceGroupNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("resourceGroupName")])
		if err != nil {
			return nil, err
		}
		hcpOpenShiftClusterNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("hcpOpenShiftClusterName")])
		if err != nil {
			return nil, err
		}
		nodePoolNameParam, err := url.PathUnescape(matches[regex.SubexpIndex("nodePoolName")])
		if err != nil {
			return nil, err
		}
		respr, errRespr := n.srv.BeginUpdate(req.Context(), resourceGroupNameParam, hcpOpenShiftClusterNameParam, nodePoolNameParam, body, nil)
		if respErr := server.GetError(errRespr, req); respErr != nil {
			return nil, respErr
		}
		beginUpdate = &respr
		n.beginUpdate.add(req, beginUpdate)
	}

	resp, err := server.PollerResponderNext(beginUpdate, req)
	if err != nil {
		return nil, err
	}

	if !contains([]int{http.StatusOK, http.StatusAccepted}, resp.StatusCode) {
		n.beginUpdate.remove(req)
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusOK, http.StatusAccepted", resp.StatusCode)}
	}
	if !server.PollerResponderMore(beginUpdate) {
		n.beginUpdate.remove(req)
	}

	return resp, nil
}
