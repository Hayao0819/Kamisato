package handler

import (
	"time"

	"github.com/Hayao0819/Kamisato/internal/mikoapi"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func domainBuildRequest(request *mikoapi.BuildRequest) *domain.BuildRequest {
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
		Repo:          request.Repo,
		Arch:          request.Arch,
		Microarch:     request.Microarch,
		Git:           git,
		Pkgbuild:      request.Pkgbuild,
		Files:         request.Files,
		InstallPkgs:   request.InstallPkgs,
		SignMode:      request.SignMode,
		Timeout:       request.Timeout,
		IgnoreArch:    request.IgnoreArch,
		RunCheck:      request.RunCheck,
		RunVerify:     request.RunVerify,
		SkipChecksums: request.SkipChecksums,
		SkipPGPCheck:  request.SkipPGPCheck,
	}
}

func protocolBuildJob(job *domain.BuildJob) mikoapi.BuildJob {
	return mikoapi.BuildJob{
		ID:        job.ID,
		Repo:      job.Repo,
		Arch:      job.Arch,
		Status:    mikoapi.JobStatus(job.Status),
		Logs:      job.Logs,
		Err:       job.Err,
		Packages:  job.Packages,
		Retries:   job.Retries,
		Reason:    mikoapi.BuildReason(job.Reason),
		CreatedAt: job.CreatedAt.Format(time.RFC3339Nano),
		StartedAt: protocolTime(job.StartedAt),
		EndedAt:   protocolTime(job.EndedAt),
	}
}

func protocolBuildJobs(jobs []*domain.BuildJob) []mikoapi.BuildJob {
	result := make([]mikoapi.BuildJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, protocolBuildJob(job))
	}
	return result
}

func protocolBuildStats(stats domain.BuildStats) mikoapi.BuildStats {
	counts := make(map[mikoapi.JobStatus]int, len(stats.Counts))
	for status, count := range stats.Counts {
		counts[mikoapi.JobStatus(status)] = count
	}
	return mikoapi.BuildStats{
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
