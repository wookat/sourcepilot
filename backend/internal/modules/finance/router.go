package finance

import "github.com/gin-gonic/gin"

// Register mounts the finance API under /finance.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	r := g.Group("/finance")
	r.GET("/expense-types", h.GetExpenseTypes)
	r.GET("/payments", h.ListPayments)
	r.POST("/payments", h.requireWrite(), h.CreatePayment)
	r.DELETE("/payments/:id", h.requireWrite(), h.DeletePayment)
	r.POST("/order-expenses", h.requireWrite(), h.CreateOrderExpense)
	r.DELETE("/order-expenses/:id", h.requireWrite(), h.DeleteOrderExpense)
	r.GET("/shop-expenses", h.ListShopExpenses)
	r.POST("/shop-expenses", h.requireWrite(), h.CreateShopExpense)
	r.DELETE("/shop-expenses/:id", h.requireWrite(), h.DeleteShopExpense)
	r.GET("/orders/:id/summary", h.GetOrderSummary)
	r.GET("/reconciliation", h.GetReconciliation)
	r.GET("/reconciliation/export.csv", h.ExportReconciliationCSV)
	r.GET("/report", h.GetReport)
	r.GET("/report/export.csv", h.ExportReportCSV)
}
