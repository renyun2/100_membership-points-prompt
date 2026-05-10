package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"membershippoints/internal/models"
)

type H struct{ DB *gorm.DB }

func Register(r *gin.Engine, db *gorm.DB) {
	h := &H{DB: db}
	api := r.Group("/api")
	{
		api.GET("/tier-configs", h.ListTierConfigs)
		api.PUT("/tier-configs", h.ReplaceTierConfigs)

		api.GET("/point-rules", h.GetPointRules)
		api.PUT("/point-rules", h.UpdatePointRules)

		api.GET("/members", h.ListMembers)
		api.POST("/members", h.CreateMember)
		api.GET("/members/by-phone/:phone", h.GetMemberByPhone)
		api.GET("/members/:id", h.GetMember)
		api.PATCH("/members/:id/recalc-tier", h.RecalcMemberTier)

		api.GET("/members/:id/summary", h.MemberSummary)
		api.GET("/members/:id/transactions", h.MemberTransactions)

		api.POST("/points/earn", h.EarnPoints)
		api.POST("/points/redeem", h.RedeemPoints)

		api.GET("/stats/active-members", h.StatActiveMembers)
		api.GET("/stats/monthly-trend", h.StatMonthlyTrend)
		api.GET("/stats/tier-distribution", h.StatTierDistribution)
		api.GET("/stats/top-balances", h.StatTopBalances)
	}
}

func parseDatePointer(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation(time.DateOnly, s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func resolveTier(tx *gorm.DB, lp int64) (string, error) {
	var code string
	err := tx.Model(&models.TierConfig{}).
		Where("min_lifetime_points <= ?", lp).
		Order("min_lifetime_points DESC").
		Limit(1).
		Pluck("tier_code", &code).Error
	return code, err
}

func (h *H) ListTierConfigs(c *gin.Context) {
	var list []models.TierConfig
	if err := h.DB.Order("min_lifetime_points").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

type tierUpsertBody struct {
	TierCode           string `json:"tier_code"`
	TierName           string `json:"tier_name"`
	MinLifetimePoints  int64  `json:"min_lifetime_points"`
}

func (h *H) ReplaceTierConfigs(c *gin.Context) {
	var body []tierUpsertBody
	if err := c.ShouldBindJSON(&body); err != nil || len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的等级配置"})
		return
	}
	for _, row := range body {
		tc := strings.TrimSpace(row.TierCode)
		if tc == "" || strings.TrimSpace(row.TierName) == "" || row.MinLifetimePoints < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tier_code/tier_name/min_lifetime_points"})
			return
		}
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		for _, row := range body {
			tc := strings.TrimSpace(row.TierCode)
			rowCopy := models.TierConfig{
				TierCode:          tc,
				TierName:          strings.TrimSpace(row.TierName),
				MinLifetimePoints: row.MinLifetimePoints,
			}
			if err := tx.Where("tier_code = ?", tc).
				Assign(map[string]any{
					"tier_name":           rowCopy.TierName,
					"min_lifetime_points": rowCopy.MinLifetimePoints,
				}).
				FirstOrCreate(&rowCopy).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.syncAllMemberTiers()
	var list []models.TierConfig
	_ = h.DB.Order("min_lifetime_points").Find(&list).Error
	c.JSON(http.StatusOK, list)
}

func (h *H) syncAllMemberTiers() {
	var members []models.Member
	if err := h.DB.Select("id", "lifetime_points").Find(&members).Error; err != nil {
		return
	}
	for _, mm := range members {
		tc, err := resolveTier(h.DB, mm.LifetimePoints)
		if err != nil || tc == "" {
			continue
		}
		_ = h.DB.Model(&models.Member{}).Where("id = ?", mm.ID).Update("tier_code", tc).Error
	}
}

func (h *H) GetPointRules(c *gin.Context) {
	var pr models.PointRules
	if err := h.DB.First(&pr, 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pr)
}

type pointRulesBody struct {
	SpendYuanPerPoint        float64 `json:"spend_yuan_per_point"`
	BirthdayMultiplier       int     `json:"birthday_multiplier"`
	CampaignMultiplier       int     `json:"campaign_multiplier"`
	CampaignEnd              *string `json:"campaign_end"`
	ExpiryJanClearAfterYears int     `json:"expiry_jan_clear_after_years"`
	ActiveEventName          string  `json:"active_event_name"`
}

func (h *H) UpdatePointRules(c *gin.Context) {
	var body pointRulesBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求体"})
		return
	}
	if body.SpendYuanPerPoint <= 0 || body.BirthdayMultiplier < 1 || body.CampaignMultiplier < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "规则参数需为正数"})
		return
	}
	if body.ExpiryJanClearAfterYears < 0 || body.ExpiryJanClearAfterYears > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "有效期年数不合法"})
		return
	}
	var pr models.PointRules
	if err := h.DB.First(&pr, 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pr.SpendYuanPerPoint = body.SpendYuanPerPoint
	pr.BirthdayMultiplier = body.BirthdayMultiplier
	pr.CampaignMultiplier = body.CampaignMultiplier
	pr.ExpiryJanClearAfterYears = body.ExpiryJanClearAfterYears
	pr.ActiveEventName = strings.TrimSpace(body.ActiveEventName)
	if body.CampaignEnd != nil {
		if strings.TrimSpace(*body.CampaignEnd) != "" {
			d, err := time.Parse(time.DateOnly, strings.TrimSpace(*body.CampaignEnd))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "campaign_end 日期"})
				return
			}
			pr.CampaignEnd = &d
		} else {
			pr.CampaignEnd = nil
		}
	}
	if err := h.DB.Save(&pr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pr)
}

func (h *H) ListMembers(c *gin.Context) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	size, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "50")))
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	qtxt := strings.TrimSpace(c.Query("q"))
	offset := (page - 1) * size
	tx := h.DB.Model(&models.Member{})
	if qtxt != "" {
		like := "%" + strings.ToLower(qtxt) + "%"
		tx = tx.Where("(LOWER(phone) LIKE ? OR LOWER(name) LIKE ?)", like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var rows []models.Member
	if err := tx.Order("id").Limit(size).Offset(offset).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var tiers []models.TierConfig
	_ = h.DB.Find(&tiers).Error
	names := map[string]string{}
	for _, tc := range tiers {
		names[tc.TierCode] = tc.TierName
	}
	type rowOut struct {
		models.Member
		TierName string `json:"tier_name"`
	}
	out := make([]rowOut, 0, len(rows))
	for _, m := range rows {
		out = append(out, rowOut{Member: m, TierName: names[m.TierCode]})
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": out})
}

type createMemberBody struct {
	Phone        string `json:"phone"`
	Name         string `json:"name"`
	Birthday     string `json:"birthday"`
	RegisteredAt string `json:"registered_at"`
}

func (h *H) CreateMember(c *gin.Context) {
	var body createMemberBody
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Phone) == "" || strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入手机号与姓名"})
		return
	}
	reg := time.Now()
	if strings.TrimSpace(body.RegisteredAt) != "" {
		d, err := time.ParseInLocation(time.DateOnly, strings.TrimSpace(body.RegisteredAt), time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "注册日期"})
			return
		}
		reg = d
	}
	bd, err := parseDatePointer(body.Birthday)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "生日格式"})
		return
	}
	m := models.Member{
		Phone:        strings.TrimSpace(body.Phone),
		Name:         strings.TrimSpace(body.Name),
		Birthday:     bd,
		RegisteredAt: reg,
		TierCode:     "normal",
	}
	if err := h.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "手机号可能已存在"})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *H) GetMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id"})
		return
	}
	var m models.Member
	if err := h.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *H) GetMemberByPhone(c *gin.Context) {
	p := strings.TrimSpace(c.Param("phone"))
	var m models.Member
	if err := h.DB.Where("phone = ?", p).First(&m).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *H) RecalcMemberTier(c *gin.Context) {
	idu, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idu == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id"})
		return
	}
	id := uint(idu)
	var m models.Member
	if err := h.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
		return
	}
	tc, err := resolveTier(h.DB, m.LifetimePoints)
	if err != nil || tc == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未能匹配等级"})
		return
	}
	if err := h.DB.Model(&m).Update("tier_code", tc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	m.TierCode = tc
	c.JSON(http.StatusOK, m)
}

func (h *H) expiryForOccurredLocal(occurred time.Time, years int) time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t := occurred.In(loc)
	anchor := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, loc)
	return anchor.AddDate(years+1, 0, 0)
}

func (h *H) MemberSummary(c *gin.Context) {
	idu, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idu == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id"})
		return
	}
	id := uint(idu)
	var m models.Member
	if err := h.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
		return
	}
	start := todayLocal()
	end := start.AddDate(0, 0, 30)
	var expSum int64
	if err := h.DB.Model(&models.PointTransaction{}).
		Where("member_id = ? AND direction = ? AND expires_at IS NOT NULL AND expires_at BETWEEN ? AND ?", id, "in", start, end).
		Select("COALESCE(SUM(points),0)").Scan(&expSum).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var tc models.TierConfig
	_ = h.DB.Where("tier_code = ?", m.TierCode).First(&tc).Error
	c.JSON(http.StatusOK, gin.H{
		"member":                     m,
		"tier_name":                  tc.TierName,
		"approx_expiring_within_30d": expSum,
	})
}

func todayLocal() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	n := time.Now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

func (h *H) MemberTransactions(c *gin.Context) {
	idu, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idu == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id"})
		return
	}
	id := uint(idu)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 30
	}
	var total int64
	if err := h.DB.Model(&models.PointTransaction{}).Where("member_id = ?", id).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var pts []models.PointTransaction
	off := (page - 1) * size
	if err := h.DB.Where("member_id = ?", id).Order("occurred_at desc, id desc").Limit(size).Offset(off).Find(&pts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": pts})
}

type earnBody struct {
	MemberID           uint    `json:"member_id"`
	SourceType         string  `json:"source_type"`
	MoneyAmount        *float64 `json:"money_amount"`
	Points             *int64  `json:"points"`
	Reason             string  `json:"reason"`
	Operator           string  `json:"operator"`
	OccurredAt         *string `json:"occurred_at"`
	UseBirthdayBonus   *bool   `json:"use_birthday_bonus"`
	UseCampaignBonus   *bool   `json:"use_campaign_bonus"`
}

func (h *H) EarnPoints(c *gin.Context) {
	var body earnBody
	if err := c.ShouldBindJSON(&body); err != nil || body.MemberID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "member_id"})
		return
	}
	op := strings.TrimSpace(body.Operator)
	if op == "" {
		op = "system"
	}
	st := strings.TrimSpace(body.SourceType)
	if st == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_type"})
		return
	}
	var rules models.PointRules
	if err := h.DB.First(&rules, 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	when := time.Now()
	if body.OccurredAt != nil && strings.TrimSpace(*body.OccurredAt) != "" {
		t, err := time.ParseInLocation(time.DateOnly, strings.TrimSpace(*body.OccurredAt), time.Local)
		if err != nil {
			t2, err2 := time.Parse(time.RFC3339, strings.TrimSpace(*body.OccurredAt))
			if err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at"})
				return
			}
			when = t2
		} else {
			when = t
		}
	}

	var pts int64
	switch st {
	case "consumption":
		if body.MoneyAmount == nil || *body.MoneyAmount <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "消费入账需要 money_amount"})
			return
		}
		raw := math.Floor(*body.MoneyAmount / rules.SpendYuanPerPoint)
		base := int64(raw)
		if base < 0 {
			base = 0
		}
		mult := float64(1)
		useBD := body.UseBirthdayBonus == nil || *body.UseBirthdayBonus
		useCM := body.UseCampaignBonus == nil || *body.UseCampaignBonus
		var mem models.Member
		if err := h.DB.First(&mem, body.MemberID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
			return
		}
		if useBD && isBirthdayLocal(mem.Birthday, when) && rules.BirthdayMultiplier > 1 {
			mult *= float64(rules.BirthdayMultiplier)
		}
		if useCM && rules.CampaignEnd != nil && !afterDate(when, *rules.CampaignEnd) && rules.CampaignMultiplier > 1 {
			mult *= float64(rules.CampaignMultiplier)
		}
		pts = int64(math.Floor(float64(base) * mult))
		if pts <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "按比例计算后为 0 分"})
			return
		}
	case "promo", "checkin", "manual":
		if body.Points == nil || *body.Points <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该来源需要整数 points"})
			return
		}
		mult := float64(1)
		useBD := body.UseBirthdayBonus == nil || *body.UseBirthdayBonus
		useCM := body.UseCampaignBonus == nil || *body.UseCampaignBonus
		var mem models.Member
		if err := h.DB.First(&mem, body.MemberID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "会员不存在"})
			return
		}
		if useBD && isBirthdayLocal(mem.Birthday, when) && rules.BirthdayMultiplier > 1 {
			mult *= float64(rules.BirthdayMultiplier)
		}
		if useCM && rules.CampaignEnd != nil && !afterDate(when, *rules.CampaignEnd) && rules.CampaignMultiplier > 1 {
			mult *= float64(rules.CampaignMultiplier)
		}
		pts = int64(math.Floor(float64(*body.Points) * mult))
		if pts <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "折算后为 0 分"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知的 source_type"})
		return
	}

	expires := h.expiryForOccurredLocal(when, rules.ExpiryJanClearAfterYears)

	var pt models.PointTransaction
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		var member models.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&member, body.MemberID).Error; err != nil {
			return err
		}
		delta := pts
		member.PointsBalance += delta
		member.LifetimePoints += delta
		tc, terr := resolveTier(tx, member.LifetimePoints)
		if terr != nil {
			return terr
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			reason = "积分入账"
		}
		pt = models.PointTransaction{
			MemberID:    body.MemberID,
			Direction:   "in",
			Points:      pts,
			PointsDelta: delta,
			SourceType:  st,
			Reason:      reason,
			Operator:    op,
			MoneyAmount: body.MoneyAmount,
			OccurredAt:  when,
			ExpiresAt:   &expires,
		}
		if err := tx.Create(&pt).Error; err != nil {
			return err
		}
		return tx.Model(&models.Member{}).Where("id = ?", member.ID).
			Updates(map[string]any{
				"points_balance":  member.PointsBalance,
				"lifetime_points": member.LifetimePoints,
				"tier_code":       tc,
			}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, pt)
}

func afterDate(a time.Time, d time.Time) bool {
	w := a.In(time.Local)
	end := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 999999999, time.Local)
	return w.After(end)
}

func isBirthdayLocal(birth *time.Time, when time.Time) bool {
	if birth == nil {
		return false
	}
	b := birth.Local()
	w := when.Local()
	return b.Month() == w.Month() && b.Day() == w.Day()
}

type redeemBody struct {
	MemberID     uint      `json:"member_id"`
	Points       int64     `json:"points"`
	SourceType   string    `json:"source_type"`
	RedeemAmount *float64  `json:"redeem_amount"`
	ProductName  string    `json:"product_name"`
	Reason       string    `json:"reason"`
	Operator     string    `json:"operator"`
	OccurredAt   *string   `json:"occurred_at"`
}

func (h *H) RedeemPoints(c *gin.Context) {
	var body redeemBody
	if err := c.ShouldBindJSON(&body); err != nil || body.MemberID == 0 || body.Points <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "member_id/points"})
		return
	}
	op := strings.TrimSpace(body.Operator)
	if op == "" {
		op = "system"
	}
	st := strings.TrimSpace(body.SourceType)
	if st == "" {
		st = "redemption"
	}
	when := time.Now()
	if body.OccurredAt != nil && strings.TrimSpace(*body.OccurredAt) != "" {
		t, err := time.ParseInLocation(time.DateOnly, strings.TrimSpace(*body.OccurredAt), time.Local)
		if err != nil {
			t2, err2 := time.Parse(time.RFC3339, strings.TrimSpace(*body.OccurredAt))
			if err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_at"})
				return
			}
			when = t2
		} else {
			when = t
		}
	}
	var pt models.PointTransaction
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var member models.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&member, body.MemberID).Error; err != nil {
			return err
		}
		if member.PointsBalance < body.Points {
			return errors.New("余额不足")
		}
		member.PointsBalance -= body.Points
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			if st == "redemption" {
				reason = "积分兑换"
			} else {
				reason = "积分抵扣"
			}
		}
		pt = models.PointTransaction{
			MemberID:       body.MemberID,
			Direction:      "out",
			Points:         body.Points,
			PointsDelta:    -body.Points,
			SourceType:     st,
			Reason:         reason,
			Operator:       op,
			RedeemAmount:   body.RedeemAmount,
			ProductName:    strings.TrimSpace(body.ProductName),
			OccurredAt:     when,
		}
		if err := tx.Create(&pt).Error; err != nil {
			return err
		}
		return tx.Model(&models.Member{}).Where("id = ?", member.ID).Update("points_balance", member.PointsBalance).Error
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "余额不足") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, pt)
}

func (h *H) StatActiveMembers(c *gin.Context) {
	since := time.Now().AddDate(0, 0, -30)
	var n int64
	raw := `
SELECT COUNT(*) FROM (
  SELECT DISTINCT member_id FROM point_transactions WHERE occurred_at >= ?
) AS t`
	if err := h.DB.Raw(raw, since).Scan(&n).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"active_members_last_30d": n})
}

type trendRow struct {
	Month           time.Time `json:"month"`
	Issued          int64     `json:"issued"`
	Redeemed        int64     `json:"redeemed"`
	ExpiredAdjusted int64     `json:"expired_adjusted"`
}

func (h *H) StatMonthlyTrend(c *gin.Context) {
	var rows []trendRow
	q := `
SELECT
  DATE_TRUNC('month', occurred_at AT TIME ZONE 'Asia/Shanghai')::DATE AS month,
  COALESCE(SUM(CASE WHEN direction = 'in' THEN points ELSE 0 END), 0)::BIGINT AS issued,
  COALESCE(SUM(CASE WHEN direction = 'out' AND source_type IN ('redemption','deduct') THEN points ELSE 0 END), 0)::BIGINT AS redeemed,
  COALESCE(SUM(CASE WHEN direction = 'out' AND source_type = 'expiry' THEN points ELSE 0 END), 0)::BIGINT AS expired_adjusted
FROM point_transactions
WHERE occurred_at >= (NOW() - INTERVAL '366 days')
GROUP BY 1
ORDER BY 1;
`
	if err := h.DB.Raw(q).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *H) StatTierDistribution(c *gin.Context) {
	type row struct {
		TierCode string `json:"tier_code"`
		N        int64  `json:"n"`
		Name     string `json:"tier_name"`
	}
	var raw []struct {
		TierCode string
		N        int64
	}
	if err := h.DB.Model(&models.Member{}).
		Select("tier_code", "COUNT(*) as n").
		Group("tier_code").
		Scan(&raw).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var tiers []models.TierConfig
	_ = h.DB.Order("min_lifetime_points").Find(&tiers).Error
	tcName := map[string]string{}
	for _, t := range tiers {
		tcName[t.TierCode] = t.TierName
	}
	out := make([]row, 0, len(raw))
	for _, r := range raw {
		out = append(out, row{TierCode: r.TierCode, N: r.N, Name: tcName[r.TierCode]})
	}
	c.JSON(http.StatusOK, out)
}

func (h *H) StatTopBalances(c *gin.Context) {
	limit := 100
	type rowOut struct {
		models.Member
		TierName string `json:"tier_name"`
	}
	var mems []models.Member
	if err := h.DB.Order("points_balance DESC, id ASC").Limit(limit).Find(&mems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var tiers []models.TierConfig
	_ = h.DB.Find(&tiers).Error
	names := map[string]string{}
	for _, tc := range tiers {
		names[tc.TierCode] = tc.TierName
	}
	list := make([]rowOut, 0, len(mems))
	for _, m := range mems {
		list = append(list, rowOut{Member: m, TierName: names[m.TierCode]})
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}
