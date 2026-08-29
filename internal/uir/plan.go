package uir

import "sync"

type FieldMapping struct {
	SourceKey string
	TargetKey string
	Target    *Node
}

type TransformationPlan struct {
	SourceName string
	TargetName string
	Fields     []FieldMapping
	Options    ProjectOptions
}

var planCache sync.Map

func CompilePlan(source, target *Node, opts ProjectOptions) *TransformationPlan {
	plan := &TransformationPlan{Options: opts}
	if source != nil {
		plan.SourceName = source.Key
	}
	if target != nil {
		plan.TargetName = target.Key
		for _, f := range target.Children {
			plan.Fields = append(plan.Fields, FieldMapping{SourceKey: f.Key, TargetKey: f.Key, Target: f})
		}
	}
	key := plan.SourceName + "->" + plan.TargetName
	planCache.Store(key, plan)
	return plan
}

func ApplyPlan(data *Node, plan *TransformationPlan) (*Node, error) {
	if plan == nil || plan.TargetName == "" {
		return data, nil
	}
	schema := NewNode(TypeMap, plan.TargetName, nil)
	for _, f := range plan.Fields {
		schema.AddChild(f.Target)
	}
	return Project(data, schema, plan.Options)
}
