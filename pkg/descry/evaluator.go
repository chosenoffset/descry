package descry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/actions"
	"github.com/chosenoffset/descry/pkg/descry/metrics"
	"github.com/chosenoffset/descry/pkg/descry/parser"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type ObjectType string

const (
	INTEGER_OBJ       = "INTEGER"
	FLOAT_OBJ         = "FLOAT"
	BOOLEAN_OBJ       = "BOOLEAN"
	STRING_OBJ        = "STRING"
	NULL_OBJ          = "NULL"
	ERROR_OBJ         = "ERROR"
	RULE_TRIGGERED_OBJ = "RULE_TRIGGERED"
)

type Integer struct {
	Value int64
}

func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) Type() ObjectType { return INTEGER_OBJ }

type Float struct {
	Value float64
}

func (f *Float) Inspect() string  { return fmt.Sprintf("%f", f.Value) }
func (f *Float) Type() ObjectType { return FLOAT_OBJ }

type Boolean struct {
	Value bool
}

func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }

type String struct {
	Value string
}

func (s *String) Inspect() string  { return s.Value }
func (s *String) Type() ObjectType { return STRING_OBJ }

type RuleTriggered struct{}

func (rt *RuleTriggered) Inspect() string  { return "rule_triggered" }
func (rt *RuleTriggered) Type() ObjectType { return RULE_TRIGGERED_OBJ }

type Null struct{}

func (n *Null) Inspect() string  { return "null" }
func (n *Null) Type() ObjectType { return NULL_OBJ }

type Error struct {
	Message string
}

func (e *Error) Inspect() string  { return "ERROR: " + e.Message }
func (e *Error) Type() ObjectType { return ERROR_OBJ }

type Evaluator struct {
	engine          *Engine
	mutex           sync.RWMutex
	currentRuleName string
}

func NewEvaluator(engine *Engine) *Evaluator {
	return &Evaluator{
		engine: engine,
	}
}

func (e *Evaluator) SetCurrentRuleName(name string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.currentRuleName = name
}

func (e *Evaluator) getCurrentRuleName() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.currentRuleName
}

func (e *Evaluator) Eval(node parser.Node) Object {
	// Use background context for backward compatibility
	return e.EvalWithContext(context.Background(), node)
}

func (e *Evaluator) EvalWithContext(ctx context.Context, node parser.Node) Object {
	// Check for context cancellation before each evaluation step
	select {
	case <-ctx.Done():
		return &Error{Message: fmt.Sprintf("evaluation cancelled: %v", ctx.Err())}
	default:
	}
	
	// Don't hold lock during evaluation - only when accessing shared state
	switch node := node.(type) {
	case *parser.Program:
		return e.evalProgramWithContext(ctx, node.Statements)

	case *parser.WhenStatement:
		return e.evalWhenStatementWithContext(ctx, node)

	case *parser.ExpressionStatement:
		return e.EvalWithContext(ctx, node.Expression)

	case *parser.BlockStatement:
		return e.evalBlockStatementWithContext(ctx, node.Statements)

	case *parser.InfixExpression:
		left := e.EvalWithContext(ctx, node.Left)
		if isError(left) {
			return left
		}
		right := e.EvalWithContext(ctx, node.Right)
		if isError(right) {
			return right
		}
		return e.evalInfixExpression(node.Operator, left, right)

	case *parser.DotExpression:
		return e.evalDotExpression(node)

	case *parser.CallExpression:
		return e.evalCallExpression(node)

	case *parser.Identifier:
		return e.evalIdentifier(node)

	case *parser.IntegerLiteral:
		return &Integer{Value: node.Value}

	case *parser.FloatLiteral:
		return &Float{Value: node.Value}

	case *parser.StringLiteral:
		return &String{Value: node.Value}

	case *parser.UnitExpression:
		return e.evalUnitExpression(node)

	default:
		return newError("unknown node type: %T", node)
	}
}

func (e *Evaluator) evalProgram(stmts []parser.Statement) Object {
	var result Object

	for _, statement := range stmts {
		result = e.Eval(statement)

		if result != nil && result.Type() == ERROR_OBJ {
			return result
		}
	}

	return result
}

func (e *Evaluator) evalProgramWithContext(ctx context.Context, stmts []parser.Statement) Object {
	var result Object

	for _, statement := range stmts {
		// Check context cancellation between statements
		select {
		case <-ctx.Done():
			return &Error{Message: fmt.Sprintf("program evaluation cancelled: %v", ctx.Err())}
		default:
		}
		
		result = e.EvalWithContext(ctx, statement)

		if result != nil && result.Type() == ERROR_OBJ {
			return result
		}
	}

	return result
}

func (e *Evaluator) evalWhenStatement(node *parser.WhenStatement) Object {
	condition := e.Eval(node.Condition)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		result := e.Eval(node.Body)
		if isError(result) {
			return result
		}
		// Return a special indicator that the rule was triggered
		return RULE_TRIGGERED
	}

	return NULL
}

func (e *Evaluator) evalWhenStatementWithContext(ctx context.Context, node *parser.WhenStatement) Object {
	// Check context before condition evaluation
	select {
	case <-ctx.Done():
		return &Error{Message: fmt.Sprintf("when statement evaluation cancelled: %v", ctx.Err())}
	default:
	}
	
	condition := e.EvalWithContext(ctx, node.Condition)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		// Check context before body evaluation
		select {
		case <-ctx.Done():
			return &Error{Message: fmt.Sprintf("when statement body evaluation cancelled: %v", ctx.Err())}
		default:
		}
		
		result := e.EvalWithContext(ctx, node.Body)
		if isError(result) {
			return result
		}
		// Return a special indicator that the rule was triggered
		return RULE_TRIGGERED
	}

	return NULL
}

func (e *Evaluator) evalBlockStatement(stmts []parser.Statement) Object {
	var result Object

	for _, statement := range stmts {
		result = e.Eval(statement)

		if result != nil && result.Type() == ERROR_OBJ {
			return result
		}
	}

	return result
}

func (e *Evaluator) evalBlockStatementWithContext(ctx context.Context, stmts []parser.Statement) Object {
	var result Object

	for _, statement := range stmts {
		// Check context cancellation between statements
		select {
		case <-ctx.Done():
			return &Error{Message: fmt.Sprintf("block statement evaluation cancelled: %v", ctx.Err())}
		default:
		}
		
		result = e.EvalWithContext(ctx, statement)

		if result != nil && result.Type() == ERROR_OBJ {
			return result
		}
	}

	return result
}

func (e *Evaluator) evalInfixExpression(operator string, left, right Object) Object {
	switch {
	case left.Type() == INTEGER_OBJ && right.Type() == INTEGER_OBJ:
		return e.evalIntegerInfixExpression(operator, left, right)
	case left.Type() == FLOAT_OBJ || right.Type() == FLOAT_OBJ:
		return e.evalFloatInfixExpression(operator, left, right)
	case left.Type() == BOOLEAN_OBJ && right.Type() == BOOLEAN_OBJ:
		return e.evalBooleanInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToPyObject(left == right)
	case operator == "!=":
		return nativeBoolToPyObject(left != right)
	default:
		return newError("unknown operator: %s", operator)
	}
}

func (e *Evaluator) evalIntegerInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Integer).Value
	rightVal := right.(*Integer).Value

	switch operator {
	case "+":
		return &Integer{Value: leftVal + rightVal}
	case "-":
		return &Integer{Value: leftVal - rightVal}
	case "*":
		return &Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		return &Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToPyObject(leftVal < rightVal)
	case ">":
		return nativeBoolToPyObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToPyObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToPyObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToPyObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToPyObject(leftVal != rightVal)
	case "&&":
		return nativeBoolToPyObject(leftVal != 0 && rightVal != 0)
	case "||":
		return nativeBoolToPyObject(leftVal != 0 || rightVal != 0)
	default:
		return newError("unknown operator: %s", operator)
	}
}

func (e *Evaluator) evalFloatInfixExpression(operator string, left, right Object) Object {
	leftVal := e.objectToFloat(left)
	rightVal := e.objectToFloat(right)

	switch operator {
	case "+":
		return &Float{Value: leftVal + rightVal}
	case "-":
		return &Float{Value: leftVal - rightVal}
	case "*":
		return &Float{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		return &Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToPyObject(leftVal < rightVal)
	case ">":
		return nativeBoolToPyObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToPyObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToPyObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToPyObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToPyObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s", operator)
	}
}

func (e *Evaluator) evalBooleanInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Boolean).Value
	rightVal := right.(*Boolean).Value

	switch operator {
	case "&&":
		return nativeBoolToPyObject(leftVal && rightVal)
	case "||":
		return nativeBoolToPyObject(leftVal || rightVal)
	case "==":
		return nativeBoolToPyObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToPyObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s", operator)
	}
}

func (e *Evaluator) evalDotExpression(node *parser.DotExpression) Object {
	// Handle metric access like heap.alloc, goroutines.count
	// Don't evaluate the left side separately - just extract the identifiers
	if leftIdent, ok := node.Left.(*parser.Identifier); ok {
		if rightIdent, ok := node.Right.(*parser.Identifier); ok {
			return e.getMetricValue(leftIdent.Value, rightIdent.Value)
		}
	}

	return newError("invalid dot expression: expected identifier.identifier")
}

func (e *Evaluator) evalCallExpression(node *parser.CallExpression) Object {
	if ident, ok := node.Function.(*parser.Identifier); ok {
		args := e.evalExpressions(node.Arguments)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		return e.callFunction(ident.Value, args)
	}

	return newError("invalid function call")
}

func (e *Evaluator) evalExpressions(exps []parser.Expression) []Object {
	var result []Object

	for _, exp := range exps {
		evaluated := e.Eval(exp)
		if isError(evaluated) {
			return []Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func (e *Evaluator) callFunction(name string, args []Object) Object {
	switch name {
	case "alert":
		if len(args) != 1 {
			return newError("wrong number of arguments for alert: got=%d, want=1", len(args))
		}
		return e.handleAlert(args[0])
	case "log":
		if len(args) != 1 {
			return newError("wrong number of arguments for log: got=%d, want=1", len(args))
		}
		return e.handleLog(args[0])
	case "avg":
		if len(args) != 2 {
			return newError("wrong number of arguments for avg: got=%d, want=2", len(args))
		}
		return e.handleAvg(args[0], args[1])
	case "max":
		if len(args) != 2 {
			return newError("wrong number of arguments for max: got=%d, want=2", len(args))
		}
		return e.handleMax(args[0], args[1])
	case "trend":
		if len(args) != 2 {
			return newError("wrong number of arguments for trend: got=%d, want=2", len(args))
		}
		return e.handleTrend(args[0], args[1])
	default:
		return newError("unknown function: %s", name)
	}
}

func (e *Evaluator) handleAlert(arg Object) Object {
	message := arg.Inspect()
	ruleName := e.getCurrentRuleName() // Safe access with proper locking
	action := e.engine.actionRegistry.CreateAction(actions.AlertAction, message, ruleName)
	
	if err := e.engine.actionRegistry.ExecuteAction(action); err != nil {
		return newError("failed to execute alert action: %s", err.Error())
	}
	
	return NULL
}

func (e *Evaluator) handleLog(arg Object) Object {
	message := arg.Inspect()
	ruleName := e.getCurrentRuleName() // Safe access with proper locking
	action := e.engine.actionRegistry.CreateAction(actions.LogAction, message, ruleName)
	
	if err := e.engine.actionRegistry.ExecuteAction(action); err != nil {
		return newError("failed to execute log action: %s", err.Error())
	}
	
	return NULL
}

func (e *Evaluator) handleAvg(metricObj, durationObj Object) Object {
	// Extract metric path from first argument (should be like "heap.alloc")
	metricPath, ok := e.extractMetricPath(metricObj)
	if !ok {
		return newError("first argument to avg() must be a metric path")
	}
	
	// Extract duration from second argument (should be like "5m" converted to duration)
	duration, ok := e.extractDuration(durationObj)
	if !ok {
		return newError("second argument to avg() must be a time duration")
	}
	
	return e.calculateMetricAverage(metricPath, duration)
}

func (e *Evaluator) handleMax(metricObj, durationObj Object) Object {
	// Extract metric path from first argument
	metricPath, ok := e.extractMetricPath(metricObj)
	if !ok {
		return newError("first argument to max() must be a metric path")
	}
	
	// Extract duration from second argument
	duration, ok := e.extractDuration(durationObj)
	if !ok {
		return newError("second argument to max() must be a time duration")
	}
	
	return e.calculateMetricMax(metricPath, duration)
}

func (e *Evaluator) handleTrend(metricObj, durationObj Object) Object {
	// Extract metric path from first argument
	metricPath, ok := e.extractMetricPath(metricObj)
	if !ok {
		return newError("first argument to trend() must be a metric path")
	}
	
	// Extract duration from second argument
	duration, ok := e.extractDuration(durationObj)
	if !ok {
		return newError("second argument to trend() must be a time duration")
	}
	
	return e.calculateMetricTrend(metricPath, duration)
}

func (e *Evaluator) extractMetricPath(obj Object) (string, bool) {
	if str, ok := obj.(*String); ok {
		return str.Value, true
	}
	return "", false
}

func (e *Evaluator) extractDuration(obj Object) (time.Duration, bool) {
	// For now, assume duration is provided as integer seconds
	// In a full implementation, this would parse unit expressions like "5m"
	switch o := obj.(type) {
	case *Integer:
		return time.Duration(o.Value) * time.Second, true
	case *Float:
		return time.Duration(o.Value * float64(time.Second)), true
	default:
		return 0, false
	}
}

func (e *Evaluator) calculateMetricAverage(metricPath string, duration time.Duration) Object {
	parts := strings.Split(metricPath, ".")
	if len(parts) != 2 {
		return newError("metric path must be in format 'category.metric'")
	}
	
	category, metric := parts[0], parts[1]
	
	// Get historical data for the specified duration
	history := e.engine.runtimeCollector.GetHistoryWindow(duration)
	if len(history) == 0 {
		return &Float{Value: 0}
	}
	
	var sum float64
	var count int
	
	for _, h := range history {
		value := e.getHistoricalMetricValue(category, metric, &h)
		if value != nil {
			sum += e.objectToFloat(value)
			count++
		}
	}
	
	if count == 0 {
		return &Float{Value: 0}
	}
	
	return &Float{Value: sum / float64(count)}
}

func (e *Evaluator) calculateMetricMax(metricPath string, duration time.Duration) Object {
	parts := strings.Split(metricPath, ".")
	if len(parts) != 2 {
		return newError("metric path must be in format 'category.metric'")
	}
	
	category, metric := parts[0], parts[1]
	
	// Get historical data for the specified duration
	history := e.engine.runtimeCollector.GetHistoryWindow(duration)
	if len(history) == 0 {
		return &Float{Value: 0}
	}
	
	var max float64
	first := true
	
	for _, h := range history {
		value := e.getHistoricalMetricValue(category, metric, &h)
		if value != nil {
			val := e.objectToFloat(value)
			if first || val > max {
				max = val
				first = false
			}
		}
	}
	
	return &Float{Value: max}
}

func (e *Evaluator) calculateMetricTrend(metricPath string, duration time.Duration) Object {
	parts := strings.Split(metricPath, ".")
	if len(parts) != 2 {
		return newError("metric path must be in format 'category.metric'")
	}
	
	category, metric := parts[0], parts[1]
	
	// Get historical data for the specified duration
	history := e.engine.runtimeCollector.GetHistoryWindow(duration)
	if len(history) < 2 {
		return &Float{Value: 0}
	}
	
	// Calculate trend as the difference between latest and earliest values
	earliest := e.getHistoricalMetricValue(category, metric, &history[0])
	latest := e.getHistoricalMetricValue(category, metric, &history[len(history)-1])
	
	if earliest == nil || latest == nil {
		return &Float{Value: 0}
	}
	
	earliestVal := e.objectToFloat(earliest)
	latestVal := e.objectToFloat(latest)
	
	// Return the rate of change per minute
	timeDiff := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
	minutesDiff := timeDiff.Minutes()
	if minutesDiff == 0 {
		return &Float{Value: 0}
	}
	
	changeRate := (latestVal - earliestVal) / minutesDiff
	return &Float{Value: changeRate}
}

func (e *Evaluator) getHistoricalMetricValue(category, metric string, runtimeMetrics *metrics.RuntimeMetrics) Object {
	// Similar to getMetricValue but works with historical data
	switch category {
	case "heap":
		switch metric {
		case "alloc":
			return &Integer{Value: int64(runtimeMetrics.HeapAlloc)}
		case "sys":
			return &Integer{Value: int64(runtimeMetrics.HeapSys)}
		case "idle":
			return &Integer{Value: int64(runtimeMetrics.HeapIdle)}
		case "inuse":
			return &Integer{Value: int64(runtimeMetrics.HeapInuse)}
		case "released":
			return &Integer{Value: int64(runtimeMetrics.HeapReleased)}
		}
	case "goroutines":
		switch metric {
		case "count":
			return &Integer{Value: int64(runtimeMetrics.NumGoroutine)}
		}
	case "gc":
		switch metric {
		case "pause":
			return &Float{Value: float64(runtimeMetrics.PauseTotalNs) / 1000000} // Convert nanoseconds to ms
		case "num":
			return &Integer{Value: int64(runtimeMetrics.NumGC)}
		}
	}
	
	return nil
}

func (e *Evaluator) evalIdentifier(node *parser.Identifier) Object {
	// For now, identifiers are not supported without dot notation
	return newError("identifier not found: %s", node.Value)
}

func (e *Evaluator) evalUnitExpression(node *parser.UnitExpression) Object {
	value := e.Eval(node.Value)
	if isError(value) {
		return value
	}

	multiplier := e.getUnitMultiplier(node.Unit)
	if multiplier == 0 {
		return newError("unknown unit: %s", node.Unit)
	}

	switch v := value.(type) {
	case *Integer:
		return &Integer{Value: v.Value * int64(multiplier)}
	case *Float:
		return &Float{Value: v.Value * multiplier}
	default:
		return newError("invalid value type for unit expression")
	}
}

func (e *Evaluator) getMetricValue(category, metric string) Object {
	runtimeMetrics := e.engine.GetRuntimeMetrics()
	httpStats := e.engine.GetHTTPMetrics()

	switch category {
	case "heap":
		switch metric {
		case "alloc":
			return &Integer{Value: int64(runtimeMetrics.HeapAlloc)}
		case "sys":
			return &Integer{Value: int64(runtimeMetrics.HeapSys)}
		case "idle":
			return &Integer{Value: int64(runtimeMetrics.HeapIdle)}
		case "inuse":
			return &Integer{Value: int64(runtimeMetrics.HeapInuse)}
		case "released":
			return &Integer{Value: int64(runtimeMetrics.HeapReleased)}
		case "objects":
			return &Integer{Value: int64(runtimeMetrics.HeapObjects)}
		}
	case "goroutines":
		switch metric {
		case "count":
			return &Integer{Value: int64(runtimeMetrics.NumGoroutine)}
		}
	case "gc":
		switch metric {
		case "pause":
			return &Float{Value: float64(runtimeMetrics.PauseTotalNs) / 1000000} // Convert nanoseconds to ms
		case "num":
			return &Integer{Value: int64(runtimeMetrics.NumGC)}
		case "cpu_fraction":
			return &Float{Value: runtimeMetrics.GCCPUFraction}
		}
	case "http":
		switch metric {
		case "request_count":
			return &Integer{Value: httpStats.RequestCount}
		case "error_count":
			return &Integer{Value: httpStats.ErrorCount}
		case "error_rate":
			return &Float{Value: httpStats.ErrorRate}
		case "request_rate":
			return &Float{Value: httpStats.RequestRate}
		case "response_time":
			return &Float{Value: float64(httpStats.AvgResponseTime) / 1000000} // Convert nanoseconds to ms
		case "max_response_time":
			return &Float{Value: float64(httpStats.MaxResponseTime) / 1000000} // Convert nanoseconds to ms
		case "pending_requests":
			return &Integer{Value: httpStats.PendingRequests}
		}
	}

	return newError("unknown metric: %s.%s", category, metric)
}

func (e *Evaluator) getUnitMultiplier(unit string) float64 {
	switch strings.ToUpper(unit) {
	case "B":
		return 1
	case "KB":
		return 1024
	case "MB":
		return 1024 * 1024
	case "GB":
		return 1024 * 1024 * 1024
	case "MS":
		return 1
	case "S":
		return 1000
	case "M":
		return 1000 * 60
	case "H":
		return 1000 * 60 * 60
	default:
		return 0
	}
}

func (e *Evaluator) objectToFloat(obj Object) float64 {
	switch o := obj.(type) {
	case *Integer:
		return float64(o.Value)
	case *Float:
		return o.Value
	default:
		return 0
	}
}

func isError(obj Object) bool {
	if obj != nil {
		return obj.Type() == ERROR_OBJ
	}
	return false
}

func isTruthy(obj Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		if boolean, ok := obj.(*Boolean); ok {
			return boolean.Value
		}
		return true
	}
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func nativeBoolToPyObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

var (
	NULL           = &Null{}
	TRUE           = &Boolean{Value: true}
	FALSE          = &Boolean{Value: false}
	RULE_TRIGGERED = &RuleTriggered{}
)