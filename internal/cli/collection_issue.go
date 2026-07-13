package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	smithy "github.com/aws/smithy-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type collectionFailureClass string

const (
	failureTimeout             collectionFailureClass = "timeout"
	failureCancelled           collectionFailureClass = "cancelled"
	failurePermissionDenied    collectionFailureClass = "permission-denied"
	failureResourceUnavailable collectionFailureClass = "resource-api-unavailable"
	failureMalformedInput      collectionFailureClass = "malformed-input"
	failureHelmProcess         collectionFailureClass = "helm-process-failure"
	failureAWSProvider         collectionFailureClass = "aws-provider-failure"
	failureUnknown             collectionFailureClass = "unknown-runtime-failure"
)

type collectionIssue struct {
	Plane                string
	Operation            string
	Class                collectionFailureClass
	Message              string
	TimedOut             bool
	Cancelled            bool
	PermissionDenied     bool
	Retryable            bool
	PartialDataPreserved bool
}

func classifyCollectionIssue(plane, operation string, err error) collectionIssue {
	issue := collectionIssue{
		Plane:                plane,
		Operation:            operation,
		Class:                failureUnknown,
		Message:              safeCollectionError(err),
		PartialDataPreserved: true,
	}
	if err == nil {
		return issue
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		issue.Class = failureTimeout
		issue.TimedOut = true
		issue.Retryable = true
	case errors.Is(err, context.Canceled):
		issue.Class = failureCancelled
		issue.Cancelled = true
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		issue.Class = failurePermissionDenied
		issue.PermissionDenied = true
	case apierrors.IsNotFound(err), apierrors.IsServiceUnavailable(err), apierrors.IsServerTimeout(err), apierrors.IsTooManyRequests(err):
		issue.Class = failureResourceUnavailable
		issue.Retryable = apierrors.IsServerTimeout(err) || apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err)
	case strings.HasPrefix(operation, "helm-chart:"):
		issue.Class = failureHelmProcess
		if errors.Is(err, context.DeadlineExceeded) {
			issue.TimedOut = true
			issue.Retryable = true
		}
	case plane == "manifests" && (strings.HasPrefix(operation, "manifest-dir:") || strings.HasPrefix(operation, "manifest-file:")):
		issue.Class = failureMalformedInput
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			issue.PermissionDenied = errors.Is(err, os.ErrPermission)
		}
	case plane == "aws":
		issue.Class = failureAWSProvider
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			code := strings.ToLower(apiErr.ErrorCode())
			if strings.Contains(code, "accessdenied") || strings.Contains(code, "unauthorized") || strings.Contains(code, "forbidden") {
				issue.Class = failurePermissionDenied
				issue.PermissionDenied = true
			}
			if strings.Contains(code, "throttl") || strings.Contains(code, "timeout") || strings.Contains(code, "unavailable") {
				issue.Retryable = true
			}
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			issue.Class = failureHelmProcess
		}
	}
	return issue
}

func (i collectionIssue) String() string {
	flags := []string{string(i.Class)}
	if i.TimedOut {
		flags = append(flags, "timedOut")
	}
	if i.Cancelled {
		flags = append(flags, "cancelled")
	}
	if i.PermissionDenied {
		flags = append(flags, "permissionDenied")
	}
	if i.Retryable {
		flags = append(flags, "retryable")
	}
	if i.PartialDataPreserved {
		flags = append(flags, "partialDataPreserved")
	}
	return fmt.Sprintf("%s [%s]: %s", i.Operation, strings.Join(flags, ","), i.Message)
}

func safeCollectionError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "collector error"
	}
	return msg
}
