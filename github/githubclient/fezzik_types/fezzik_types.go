package fezzik_types

type MergeableState string

const (
	MergeableState_CONFLICTING MergeableState = "CONFLICTING"
	MergeableState_MERGEABLE   MergeableState = "MERGEABLE"
	MergeableState_UNKNOWN     MergeableState = "UNKNOWN"
)

type PullRequestReviewDecision string

const (
	PullRequestReviewDecision_APPROVED          PullRequestReviewDecision = "APPROVED"
	PullRequestReviewDecision_CHANGES_REQUESTED PullRequestReviewDecision = "CHANGES_REQUESTED"
	PullRequestReviewDecision_REVIEW_REQUIRED   PullRequestReviewDecision = "REVIEW_REQUIRED"
)

type StatusState string

const (
	StatusState_ERROR    StatusState = "ERROR"
	StatusState_EXPECTED StatusState = "EXPECTED"
	StatusState_FAILURE  StatusState = "FAILURE"
	StatusState_PENDING  StatusState = "PENDING"
	StatusState_SUCCESS  StatusState = "SUCCESS"
)

type PullRequestConnection struct {
	Nodes *PullRequestsViewerPullRequestsNodes
}

type PullRequestsViewerPullRequestsNodes []*struct {
	Id              string
	Number          int
	Title           string
	Body            string
	BaseRefName     string
	HeadRefName     string
	Mergeable       MergeableState
	ReviewDecision  *PullRequestReviewDecision
	Repository      PullRequestsViewerPullRequestsNodesRepository
	MergeQueueEntry *PullRequestsViewerPullRequestsNodesMergeQueueEntry
	Commits         PullRequestsViewerPullRequestsNodesCommits
}

type PullRequestsViewerPullRequestsNodesRepository struct {
	Id string
}

type PullRequestsViewerPullRequestsNodesMergeQueueEntry struct {
	Id string
}

type PullRequestsViewerPullRequestsNodesCommits struct {
	Nodes *PullRequestsViewerPullRequestsNodesCommitsNodes
}

type PullRequestsViewerPullRequestsNodesCommitsNodes []*struct {
	Commit PullRequestsViewerPullRequestsNodesCommitsNodesCommit
}

type PullRequestsViewerPullRequestsNodesCommitsNodesCommit struct {
	Oid               string
	MessageHeadline   string
	MessageBody       string
	StatusCheckRollup *PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup
}

type PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup struct {
	State StatusState
}
