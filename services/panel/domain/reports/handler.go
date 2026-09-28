package reports

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	experimentsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/experiments"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log.Named("reports")}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	reports := rg.Group("/reports")
	{
		reports.GET("/:experiment_id", h.getReport)
		reports.GET("/:experiment_id/data-quality", h.getDataQuality)
	}
}

func (h *Handler) getReport(c *gin.Context) {
	experimentID, ok := parseExperimentID(c)
	if !ok {
		return
	}

	var req ReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid query parameters")
		return
	}

	if req.Interval == "" {
		req.Interval = "day"
	}
	if req.Format == "" {
		req.Format = "json"
	}

	report, err := h.svc.GetReport(c.Request.Context(), experimentID, ReportQuery{
		Start:    req.Start,
		End:      req.End,
		Interval: req.Interval,
		Metrics:  req.Metrics,
		Format:   req.Format,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	if req.Format == "csv" {
		h.writeCSV(c, report)
		return
	}

	api.OK(c.Writer, toResponse(report))
}

func (h *Handler) getDataQuality(c *gin.Context) {
	experimentID, ok := parseExperimentID(c)
	if !ok {
		return
	}

	start, err := time.Parse(time.RFC3339, c.DefaultQuery("start", ""))
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid start date")
		return
	}
	end, err := time.Parse(time.RFC3339, c.DefaultQuery("end", ""))
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid end date")
		return
	}

	dq, err := h.svc.GetDataQuality(c.Request.Context(), experimentID, start, end)
	if err != nil {
		h.respondError(c, err)
		return
	}

	api.OK(c.Writer, toDataQualityResponse(dq))
}

func (h *Handler) writeCSV(c *gin.Context, report *Report) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"metric", "variant_id", "variant_name", "timestamp", "value"})

	for _, m := range report.Metrics {
		for _, p := range m.TimeSeries {
			_ = w.Write([]string{m.Key, "", "", p.Timestamp.Format(time.RFC3339), formatFloat(p.Value)})
		}
		for _, v := range m.ByVariant {
			_ = w.Write([]string{m.Key, v.VariantID, v.VariantName, "", formatFloat(v.Value)})
		}
	}

	w.Flush()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"report_%s.csv\"", report.ExperimentID))
	c.String(http.StatusOK, buf.String())
}

func (h *Handler) respondError(c *gin.Context, err error) {
	w := c.Writer
	switch {
	case errors.Is(err, ErrInvalidInterval), errors.Is(err, ErrInvalidDateRange), errors.Is(err, ErrNoMetrics):
		api.Error(w, http.StatusBadRequest, api.BadRequest, err.Error())
	case errors.Is(err, experimentsdomain.ErrNotFound):
		api.Error(w, http.StatusNotFound, api.NotFound, "experiment not found")
	default:
		logger.SetErrorType(c, logger.ErrorTypeInternalError)
		h.log.Error("report error", zap.Error(err))
		api.InternalError(w)
	}
}

func parseExperimentID(c *gin.Context) (uuid.UUID, bool) {
	idStr := c.Param("experiment_id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "invalid experiment id")
		return uuid.Nil, false
	}
	return id, true
}

func toResponse(r *Report) ReportResponse {
	return ReportResponse{
		ExperimentID:   r.ExperimentID,
		ExperimentName: r.ExperimentName,
		Start:          r.Start,
		End:            r.End,
		Interval:       r.Interval,
		Metrics:        r.Metrics,
		SampleSize:     r.SampleSize,
		AllocationSkew: r.AllocationSkew,
	}
}

func toDataQualityResponse(dq *DataQuality) DataQualityResponse {
	return DataQualityResponse{
		RejectedRate:       dq.RejectedRate,
		DuplicateRate:      dq.DuplicateRate,
		PendingAttribution: dq.PendingAttribution,
		AttributionLagSec:  dq.AttributionLagSec,
	}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
