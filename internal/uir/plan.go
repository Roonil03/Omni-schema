package uir

import "sync"

type FieldMapping struct {
	SourceKey string
	TargetKey string
	Target    *Node
}

type TransformationPlan struct {
	Key        string
	SourceName string
	TargetName string
	Target     *Node
	Options    ProjectOptions
}

var planCache sync.Map

func PlanCacheKey(sourceSchema, sourceType, targetSchema, targetType, sourceVer, targetVer string) string {
	return sourceSchema + "@" + sourceVer + ":" + sourceType + "=>" + targetSchema + "@" + targetVer + ":" + targetType
}

func CompilePlan(source, target *Node, opts ProjectOptions) *TransformationPlan {
	plan := &TransformationPlan{Options: opts, Target: target}
	if source != nil {
		plan.SourceName = source.Key
	}
	if target != nil {
		plan.TargetName = target.Key
	}
	plan.Key = plan.SourceName + "->" + plan.TargetName
	planCache.Store(plan.Key, plan)
	return plan
}

func GetOrCompilePlan(key string, source, target *Node, opts ProjectOptions) *TransformationPlan {
	if key != "" {
		if v, ok := planCache.Load(key); ok {
			return v.(*TransformationPlan)
		}
	}
	plan := CompilePlan(source, target, opts)
	if key != "" {
		plan.Key = key
		planCache.Store(key, plan)
	}
	return plan
}

func ApplyPlan(data *Node, plan *TransformationPlan) (*Node, error) {
	if plan == nil || plan.Target == nil {
		return data, nil
	}
	return Project(data, plan.Target, plan.Options)
}
