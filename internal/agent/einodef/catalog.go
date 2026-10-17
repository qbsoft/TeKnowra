// Package einodef is the server-controlled publication manifest. Business
// definitions are reviewed and compiled; chat requests cannot register code.
package einodef

import "slices"

const ContractReview = "contract-review-v1"
const CustomerCredit = "customer-credit-readonly-v1"
const KnowledgeSummary = "knowledge-summary-v1"
const BusinessMultiAgent = "business-review-multiagent-v1"

type Definition struct {
	ID            string
	Name          string
	Knowledge     bool
	ReadOnlyTools []string
}

// Add new reviewed definitions here and their builders in the runtime manifest.
// These are business publication manifests, not dispatch switches.
var published = map[string]Definition{
	ContractReview:     {ID: ContractReview, Name: "合同审查 V2", Knowledge: true},
	CustomerCredit:     {ID: CustomerCredit, Name: "客户信用只读查询", ReadOnlyTools: []string{"find_customer", "get_customer_credit_control", "check_customer_access"}},
	KnowledgeSummary:   {ID: KnowledgeSummary, Name: "授权知识摘要", Knowledge: true},
	BusinessMultiAgent: {ID: BusinessMultiAgent, Name: "合同与信用多 Agent", Knowledge: true, ReadOnlyTools: []string{"find_customer", "get_customer_credit_control", "check_customer_access"}},
}

func Lookup(id string) (Definition, bool) {
	d, ok := published[id]
	d.ReadOnlyTools = slices.Clone(d.ReadOnlyTools)
	return d, ok
}

func IDs() []string {
	ids := make([]string, 0, len(published))
	for id := range published {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (d Definition) AllowsTool(name string) bool { return slices.Contains(d.ReadOnlyTools, name) }
