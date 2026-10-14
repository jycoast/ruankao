package service

import (
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type ExamScheduleService struct {
	repo repository.ExamScheduleRepository
}

func NewExamScheduleService(repo repository.ExamScheduleRepository) *ExamScheduleService {
	return &ExamScheduleService{repo: repo}
}

func (s *ExamScheduleService) List() ([]model.ExamSchedule, error) {
	return s.repo.List()
}

func (s *ExamScheduleService) Create(label string, examDate time.Time) (uint64, error) {
	sched := &model.ExamSchedule{
		Label:    label,
		ExamDate: examDate,
	}
	if err := s.repo.Create(sched); err != nil {
		return 0, err
	}
	return sched.ID, nil
}

func (s *ExamScheduleService) Update(id uint64, label string, examDate time.Time) error {
	sched, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	sched.Label = label
	sched.ExamDate = examDate
	return s.repo.Update(sched)
}

func (s *ExamScheduleService) Delete(id uint64) error {
	return s.repo.Delete(id)
}
