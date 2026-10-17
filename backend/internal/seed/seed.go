// Package seed provides demo data: three business flows with full node
// contracts (system, repository, params, sample response) and simulated
// traffic so the UI has something meaningful to show on first run.
package seed

import (
	"context"
	"log"

	"gorm.io/gorm"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/models"
	"flowmonitor/backend/internal/service"
)

// Demo creates demo flows (when none exist) and simulated traffic (when no
// executions exist).
func Demo(db *gorm.DB, cfg *config.Config, svc *service.Services) error {
	var flowCount int64
	if err := db.Model(&models.Flow{}).Count(&flowCount).Error; err != nil {
		return err
	}
	if flowCount == 0 {
		for _, flow := range demoFlows() {
			if err := db.Create(&flow).Error; err != nil {
				return err
			}
		}
		log.Printf("seed: created %d demo business flows", len(demoFlows()))
	}
	return seedTraffic(db, cfg, svc)
}

func seedTraffic(db *gorm.DB, cfg *config.Config, svc *service.Services) error {
	var runCount int64
	if err := db.Model(&models.Run{}).Count(&runCount).Error; err != nil {
		return err
	}
	if runCount > 0 {
		return nil
	}
	stats, err := svc.Simulate.Run(context.Background(), service.SimOptions{
		Hours:       72,
		RunsPerFlow: 45,
		ActiveRuns:  4, // executions left running, some of them stuck over SLA
		Seed:        20261017,
	})
	if err != nil {
		return err
	}
	log.Printf("seed: simulated %d executions, %d tasks, %d events (%d over SLA)",
		stats.Runs, stats.Steps, stats.Events, stats.Stuck)
	return nil
}

// ---------------------------------------------------------------------------
// demo flow definitions
// ---------------------------------------------------------------------------

// sys describes where a process step lives.
type sys struct {
	name, repo, team string
}

var (
	sysStorefront   = sys{"order-service", "github.com/acme/storefront/order-service", "Digital Channels"}
	sysPayment      = sys{"payment-service", "github.com/acme/payments/payment-service", "Payments"}
	sysInventory    = sys{"inventory-service", "github.com/acme/supply/inventory-service", "Supply Chain"}
	sysShipping     = sys{"shipping-service", "github.com/acme/logistics/shipping-service", "Logistics"}
	sysNotification = sys{"notification-service", "github.com/acme/platform/notification-service", "Platform"}
	sysCRM          = sys{"crm-service", "github.com/acme/crm/crm-service", "CRM"}
	sysKYC          = sys{"kyc-service", "github.com/acme/crm/kyc-service", "Compliance"}
	sysClaims       = sys{"claims-service", "github.com/acme/insurance/claims-service", "Claims"}
	sysPolicy       = sys{"policy-service", "github.com/acme/insurance/policy-service", "Underwriting"}
	sysAssessment   = sys{"assessment-service", "github.com/acme/insurance/assessment-service", "Claims"}
	sysFinance      = sys{"finance-service", "github.com/acme/finance/finance-service", "Finance"}
)

// n builds a node compactly.
func n(key, name string, kind models.NodeKind, x, y float64, system sys, method, endpoint string, timeout int, req, resp string) models.FlowNode {
	return models.FlowNode{
		Key:            key,
		Name:           name,
		Kind:           kind,
		System:         system.name,
		Repository:     system.repo,
		Team:           system.team,
		Method:         method,
		Endpoint:       endpoint,
		TimeoutMinutes: timeout,
		PosX:           x,
		PosY:           y,
		RequestSchema:  req,
		ResponseSample: resp,
	}
}

// e builds an edge compactly.
func e(source, target, label, condition string) models.FlowEdge {
	return models.FlowEdge{SourceKey: source, TargetKey: target, Label: label, Condition: condition}
}

func demoFlows() []models.Flow {
	return []models.Flow{demoOrderFulfillment(), demoCustomerOnboarding(), demoClaimProcessing()}
}

func demoOrderFulfillment() models.Flow {
	return models.Flow{
		Key:         "order-fulfillment",
		Name:        "Order Fulfillment",
		Description: "From order capture through payment, inventory reservation and shipment.",
		Category:    "Sales",
		Owner:       "Digital Channels",
		Version:     "1.4.0",
		Active:      true,
		Nodes: []models.FlowNode{
			n("order_received", "Order Received", models.KindStart, 40, 200, sysStorefront, "POST", "/orders", 2,
				`{"orderId":"ORD-10231","channel":"web","amount":129.9,"currency":"USD","items":2}`,
				`{"orderId":"ORD-10231","status":"RECEIVED","receivedAt":"2026-10-17T09:31:02Z"}`),
			n("validate_payment", "Validate Payment", models.KindTask, 280, 200, sysPayment, "POST", "/payments/authorize", 5,
				`{"orderId":"ORD-10231","amount":129.9,"currency":"USD","paymentMethod":"card","token":"tok_visa"}`,
				`{"authorized":true,"authCode":"A1B2C3","provider":"stripe","capturedAt":"2026-10-17T09:32:10Z"}`),
			n("payment_decision", "Payment Approved?", models.KindDecision, 520, 200, sysPayment, "POST", "/payments/decision", 3,
				`{"orderId":"ORD-10231","authorizationResult":"approved","riskScore":12}`,
				`{"approved":true,"reason":"risk score within limit"}`),
			n("reserve_inventory", "Reserve Inventory", models.KindTask, 760, 120, sysInventory, "POST", "/inventory/reservations", 10,
				`{"orderId":"ORD-10231","sku":"SKU-7781","quantity":2,"warehouse":"auto"}`,
				`{"reservationId":"RSV-88","warehouse":"WH-JKT-1","reserved":true,"expiresAt":"2026-10-18T09:00:00Z"}`),
			n("create_shipment", "Create Shipment", models.KindTask, 1000, 120, sysShipping, "POST", "/shipments", 20,
				`{"orderId":"ORD-10231","reservationId":"RSV-88","address":{"city":"Jakarta","country":"ID"},"carrier":"auto"}`,
				`{"shipmentId":"SHP-221","carrier":"JNE","trackingNumber":"JNE123456","etaDays":3}`),
			n("notify_customer", "Notify Customer", models.KindTask, 1240, 120, sysNotification, "POST", "/notifications/order-shipped", 5,
				`{"orderId":"ORD-10231","channel":"email","template":"order_shipped","trackingNumber":"JNE123456"}`,
				`{"notificationId":"NTF-9912","queued":true}`),
			n("order_completed", "Order Completed", models.KindEnd, 1480, 120, sysStorefront, "POST", "/orders/complete", 2,
				`{"orderId":"ORD-10231","status":"COMPLETED"}`,
				`{"orderId":"ORD-10231","status":"COMPLETED","closedAt":"2026-10-17T09:45:00Z"}`),
			n("order_rejected", "Order Rejected", models.KindEnd, 760, 340, sysStorefront, "POST", "/orders/reject", 2,
				`{"orderId":"ORD-10231","reason":"payment declined"}`,
				`{"orderId":"ORD-10231","status":"REJECTED"}`),
		},
		Edges: []models.FlowEdge{
			e("order_received", "validate_payment", "", ""),
			e("validate_payment", "payment_decision", "", ""),
			e("payment_decision", "reserve_inventory", "yes", "approved == true"),
			e("payment_decision", "order_rejected", "no", "approved == false"),
			e("reserve_inventory", "create_shipment", "", ""),
			e("create_shipment", "notify_customer", "", ""),
			e("notify_customer", "order_completed", "", ""),
		},
	}
}

func demoCustomerOnboarding() models.Flow {
	return models.Flow{
		Key:         "customer-onboarding",
		Name:        "Customer Onboarding",
		Description: "Application, KYC screening, optional manual review, account creation and welcome kit.",
		Category:    "Onboarding",
		Owner:       "CRM",
		Version:     "2.1.0",
		Active:      true,
		Nodes: []models.FlowNode{
			n("application_submitted", "Application Submitted", models.KindStart, 40, 200, sysCRM, "POST", "/applications", 2,
				`{"applicationId":"APP-4471","fullName":"Rina Wijaya","email":"rina@example.com","segment":"retail"}`,
				`{"applicationId":"APP-4471","status":"SUBMITTED"}`),
			n("kyc_screening", "KYC Screening", models.KindExternal, 280, 200, sysKYC, "POST", "/kyc/screenings", 15,
				`{"applicationId":"APP-4471","fullName":"Rina Wijaya","nationalId":"3174xxxx","sanctionsLists":["OFAC","EU"]}`,
				`{"screeningId":"SCR-77","matchScore":0.02,"sanctionsHit":false,"pepHit":false}`),
			n("manual_review_required", "Manual Review?", models.KindDecision, 520, 200, sysKYC, "POST", "/kyc/decision", 3,
				`{"applicationId":"APP-4471","matchScore":0.02,"annualIncome":48000}`,
				`{"manualReview":false,"reason":"low risk profile"}`),
			n("analyst_review", "Analyst Review", models.KindTask, 760, 340, sysCRM, "POST", "/applications/{id}/review", 240,
				`{"applicationId":"APP-4471","queue":"compliance","assigneeRole":"analyst"}`,
				`{"decision":"approved","analyst":"analyst.01","notes":"documents verified"}`),
			n("create_account", "Create Account", models.KindTask, 760, 120, sysCRM, "POST", "/accounts", 5,
				`{"applicationId":"APP-4471","productCode":"SAVINGS-BASIC","currency":"IDR"}`,
				`{"accountId":"ACC-99311","accountNumber":"8800123456","status":"ACTIVE"}`),
			n("send_welcome_kit", "Send Welcome Kit", models.KindTask, 1000, 120, sysNotification, "POST", "/notifications/welcome", 5,
				`{"accountId":"ACC-99311","channel":"email","template":"welcome_kit"}`,
				`{"notificationId":"NTF-5521","queued":true}`),
			n("customer_active", "Customer Active", models.KindEnd, 1240, 120, sysCRM, "POST", "/customers/activate", 2,
				`{"accountId":"ACC-99311"}`,
				`{"customerId":"CUS-3312","status":"ACTIVE"}`),
			n("application_rejected", "Application Rejected", models.KindEnd, 1000, 340, sysCRM, "POST", "/applications/reject", 2,
				`{"applicationId":"APP-4471","reason":"manual review rejected"}`,
				`{"applicationId":"APP-4471","status":"REJECTED"}`),
		},
		Edges: []models.FlowEdge{
			e("application_submitted", "kyc_screening", "", ""),
			e("kyc_screening", "manual_review_required", "", ""),
			e("manual_review_required", "analyst_review", "yes", "manualReview == true"),
			e("manual_review_required", "create_account", "no", "manualReview == false"),
			e("analyst_review", "create_account", "approved", "decision == approved"),
			e("analyst_review", "application_rejected", "rejected", "decision == rejected"),
			e("create_account", "send_welcome_kit", "", ""),
			e("send_welcome_kit", "customer_active", "", ""),
		},
	}
}

func demoClaimProcessing() models.Flow {
	return models.Flow{
		Key:         "claim-processing",
		Name:        "Insurance Claim Processing",
		Description: "Claim intake, policy verification, damage assessment, payout approval and settlement.",
		Category:    "Insurance",
		Owner:       "Claims",
		Version:     "1.0.3",
		Active:      true,
		Nodes: []models.FlowNode{
			n("claim_filed", "Claim Filed", models.KindStart, 40, 200, sysClaims, "POST", "/claims", 2,
				`{"claimId":"CLM-7742","policyNo":"POL-99812","incidentDate":"2026-10-14","amount":1250.0}`,
				`{"claimId":"CLM-7742","status":"FILED"}`),
			n("verify_policy", "Verify Policy", models.KindTask, 280, 200, sysPolicy, "GET", "/policies/{policyNo}/coverage", 8,
				`{"policyNo":"POL-99812","claimType":"accident"}`,
				`{"policyNo":"POL-99812","active":true,"coverageLimit":5000.0,"deductible":250.0}`),
			n("claim_eligible", "Claim Eligible?", models.KindDecision, 520, 200, sysPolicy, "POST", "/claims/eligibility", 3,
				`{"claimId":"CLM-7742","policyNo":"POL-99812","claimedAmount":1250.0}`,
				`{"eligible":true,"payableAmount":1000.0,"reason":"within coverage limit"}`),
			n("assess_damage", "Assess Damage", models.KindExternal, 760, 120, sysAssessment, "POST", "/assessments", 30,
				`{"claimId":"CLM-7742","claimType":"accident","documents":["photo-1.jpg","police-report.pdf"]}`,
				`{"assessmentId":"ASM-441","severity":"moderate","estimatedCost":1100.0,"adjuster":"ADJ-12"}`),
			n("payout_approval", "Payout Approval", models.KindTask, 1000, 120, sysFinance, "POST", "/payouts/approval", 60,
				`{"claimId":"CLM-7742","payableAmount":1000.0,"approvalLevel":"manual"}`,
				`{"approvalId":"APR-8871","approved":true,"approver":"finance.lead"}`),
			n("disburse_payout", "Disburse Payout", models.KindTask, 1240, 120, sysFinance, "POST", "/payouts/disbursements", 20,
				`{"approvalId":"APR-8871","bankAccount":"1234567890","amount":1000.0}`,
				`{"disbursementId":"DSB-3320","status":"SENT","provider":"bank-transfer"}`),
			n("close_claim", "Close Claim", models.KindTask, 1480, 120, sysClaims, "POST", "/claims/{id}/close", 5,
				`{"claimId":"CLM-7742","outcome":"settled"}`,
				`{"claimId":"CLM-7742","status":"CLOSED"}`),
			n("claim_settled", "Claim Settled", models.KindEnd, 1720, 120, sysClaims, "POST", "/claims/settle", 2,
				`{"claimId":"CLM-7742"}`,
				`{"claimId":"CLM-7742","status":"SETTLED","settledAt":"2026-10-17T11:00:00Z"}`),
			n("claim_denied", "Claim Denied", models.KindEnd, 1000, 340, sysClaims, "POST", "/claims/deny", 2,
				`{"claimId":"CLM-7742","reason":"not eligible"}`,
				`{"claimId":"CLM-7742","status":"DENIED"}`),
		},
		Edges: []models.FlowEdge{
			e("claim_filed", "verify_policy", "", ""),
			e("verify_policy", "claim_eligible", "", ""),
			e("claim_eligible", "assess_damage", "yes", "eligible == true"),
			e("claim_eligible", "claim_denied", "no", "eligible == false"),
			e("assess_damage", "payout_approval", "", ""),
			e("payout_approval", "disburse_payout", "approved", "approved == true"),
			e("payout_approval", "claim_denied", "denied", "approved == false"),
			e("disburse_payout", "close_claim", "", ""),
			e("close_claim", "claim_settled", "", ""),
		},
	}
}
