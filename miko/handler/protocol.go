package handler

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/protocol"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func domainBuildRequest(request *protocol.BuildRequest) *domain.BuildRequest {
	if request == nil {
		return nil
	}
	var git *domain.GitSource
	if request.Git != nil {
		git = &domain.GitSource{
			URL:    request.Git.URL,
			Ref:    request.Git.Ref,
			Subdir: request.Git.Subdir,
		}
	}
	return &domain.BuildRequest{
		Repo:        request.Repo,
		Arch:        request.Arch,
		Microarch:   request.Microarch,
		Git:         git,
		Pkgbuild:    request.Pkgbuild,
		Files:       request.Files,
		InstallPkgs: request.InstallPkgs,
		SignMode:    request.SignMode,
		Timeout:     request.Timeout,
	}
}

func protocolBuildJob(job *domain.BuildJob) protocol.BuildJob {
	return protocol.BuildJob{
		ID:        job.ID,
		Repo:      job.Repo,
		Arch:      job.Arch,
		Status:    protocol.JobStatus(job.Status),
		Logs:      job.Logs,
		Err:       job.Err,
		Packages:  job.Packages,
		Retries:   job.Retries,
		Reason:    protocol.BuildReason(job.Reason),
		CreatedAt: job.CreatedAt.Format(time.RFC3339Nano),
		StartedAt: protocolTime(job.StartedAt),
		EndedAt:   protocolTime(job.EndedAt),
	}
}

func protocolBuildJobs(jobs []*domain.BuildJob) []protocol.BuildJob {
	result := make([]protocol.BuildJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, protocolBuildJob(job))
	}
	return result
}

func protocolBuildStats(stats domain.BuildStats) protocol.BuildStats {
	counts := make(map[protocol.JobStatus]int, len(stats.Counts))
	for status, count := range stats.Counts {
		counts[protocol.JobStatus(status)] = count
	}
	return protocol.BuildStats{
		Workers:     stats.Workers,
		QueueLength: stats.QueueLength,
		Running:     stats.Running,
		Counts:      counts,
		Total:       stats.Total,
		SuccessRate: stats.SuccessRate,
		UptimeSec:   int64(stats.UptimeSec),
	}
}

func protocolTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}
