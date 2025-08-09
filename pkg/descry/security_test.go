package descry

import (
	"strings"
	"testing"
)

// WARNING: This file contains attack patterns for security testing only
// DO NOT use these patterns as examples in documentation
// These patterns test that the rules engine safely handles potentially malicious input

func TestSecurityPatterns(t *testing.T) {
	engine := NewEngine()
	
	// Test that potentially malicious patterns are safely handled
	// These patterns attempt various forms of code injection or system access
	attackPatterns := []struct {
		name        string
		rule        string
		expectError bool
		description string
	}{
		{
			name:        "Environment Variable Access",
			rule:        `when heap.alloc > 100MB { log("${os.Getenv('HOME')}") }`,
			expectError: false, // Should parse but string interpolation should be safe
			description: "Attempts to access environment variables through string interpolation",
		},
		{
			name:        "Command Execution",
			rule:        `when heap.alloc > 100MB { alert("${exec.Command('ls').Output()}") }`,
			expectError: false, // Should parse but execution should not happen
			description: "Attempts to execute system commands",
		},
		{
			name:        "File Path Traversal",
			rule:        `when heap.alloc > 100MB { log("../../../etc/passwd") }`,
			expectError: false, // Path traversal in string literals should be harmless
			description: "Contains path traversal patterns",
		},
		{
			name:        "SQL Injection Pattern",
			rule:        `when heap.alloc > 100MB { log("'; DROP TABLE users; --") }`,
			expectError: false, // SQL injection patterns in strings should be safe
			description: "Contains SQL injection patterns",
		},
		{
			name:        "Script Injection",
			rule:        `when heap.alloc > 100MB { alert("<script>alert('xss')</script>") }`,
			expectError: false, // Script tags in strings should be safe
			description: "Contains script injection patterns",
		},
		{
			name:        "Null Byte Injection",
			rule:        `when heap.alloc > 100MB { log("test\x00admin") }`,
			expectError: false, // Null bytes should be handled safely
			description: "Contains null byte injection patterns",
		},
		{
			name:        "Format String Attack",
			rule:        `when heap.alloc > 100MB { log("%n%n%n%n") }`,
			expectError: false, // Format strings should be safe in our context
			description: "Contains format string attack patterns",
		},
		{
			name:        "Buffer Overflow Pattern",
			rule:        `when heap.alloc > 100MB { alert("` + strings.Repeat("A", 10000) + `") }`,
			expectError: false, // Large strings should be handled safely
			description: "Contains very large string to test buffer handling",
		},
	}

	for _, tc := range attackPatterns {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.LoadRule("security_test_"+tc.name, tc.rule)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for malicious pattern but got none: %s", tc.description)
			} else if !tc.expectError && err != nil {
				// Only fail if it's a parse error, not a security rejection
				if strings.Contains(err.Error(), "parse errors") {
					t.Errorf("Rule should parse safely even with malicious pattern: %v\nDescription: %s", err, tc.description)
				}
			}
			
			// Clear the rule to avoid interference with other tests
			engine.ClearRules()
		})
	}
}

func TestResourceExhaustionPatterns(t *testing.T) {
	engine := NewEngine()
	
	// Test patterns that might cause resource exhaustion
	exhaustionPatterns := []struct {
		name        string
		rule        string
		expectError bool
		description string
	}{
		{
			name:        "Deep Nesting",
			rule:        `when ((((((((((heap.alloc > 0)))))))))) { log("deep nesting") }`,
			expectError: false, // Should be handled by complexity limits
			description: "Deeply nested expressions to test parser limits",
		},
		{
			name:        "Many Conditions",
			rule:        `when heap.alloc > 0 && heap.alloc > 1 && heap.alloc > 2 && heap.alloc > 3 && heap.alloc > 4 && heap.alloc > 5 && heap.alloc > 6 && heap.alloc > 7 && heap.alloc > 8 && heap.alloc > 9 { log("many conditions") }`,
			expectError: false, // Should be handled by complexity limits
			description: "Many AND conditions to test evaluation limits",
		},
		{
			name:        "Recursive Function Calls",
			rule:        `when heap.alloc > 0 { log(avg(max(trend("heap.alloc", 60), 30), 120)) }`,
			expectError: false, // Nested function calls should be safe
			description: "Nested function calls to test stack depth",
		},
	}

	for _, tc := range exhaustionPatterns {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.LoadRule("exhaustion_test_"+tc.name, tc.rule)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for resource exhaustion pattern but got none: %s", tc.description)
			} else if !tc.expectError && err != nil {
				// Check if it was rejected due to complexity limits
				if strings.Contains(err.Error(), "complexity") || strings.Contains(err.Error(), "limit") {
					t.Logf("Rule correctly rejected due to complexity limits: %v", err)
				} else {
					t.Errorf("Unexpected error for exhaustion pattern: %v\nDescription: %s", err, tc.description)
				}
			}
			
			// Clear the rule
			engine.ClearRules()
		})
	}
}

func TestSecurityLimitsEnforcement(t *testing.T) {
	// Test that security limits are properly enforced
	t.Run("RuleLimitEnforcement", func(t *testing.T) {
		engine := NewEngine()
		
		// Set very low limits
		limits := engine.GetResourceLimits()
		limits.MaxRules = 2
		limits.MaxRuleComplexity = 5
		engine.SetResourceLimits(limits)
		
		// Should be able to add up to the limit
		for i := 0; i < 2; i++ {
			rule := `when heap.alloc > 0 { log("test") }`
			err := engine.LoadRule(fmt.Sprintf("limit_test_%d", i), rule)
			if err != nil {
				t.Fatalf("Should be able to add rule within limits: %v", err)
			}
		}
		
		// Adding one more should fail
		err := engine.LoadRule("limit_test_excess", `when heap.alloc > 0 { log("excess") }`)
		if err == nil {
			t.Error("Should have failed when exceeding rule limit")
		}
	})
	
	t.Run("ComplexityLimitEnforcement", func(t *testing.T) {
		engine := NewEngine()
		
		// Set very low complexity limit
		limits := engine.GetResourceLimits()
		limits.MaxRuleComplexity = 5
		engine.SetResourceLimits(limits)
		
		// Complex rule should be rejected
		complexRule := `when heap.alloc > 100MB && goroutines.count > 100 && avg("heap.alloc", 60) > 50MB { alert("too complex") }`
		err := engine.LoadRule("complex_rule", complexRule)
		if err == nil {
			t.Error("Should have failed when exceeding complexity limit")
		}
		
		if !strings.Contains(err.Error(), "complexity") {
			t.Errorf("Expected complexity error, got: %v", err)
		}
	})
}

func TestSandboxingSafety(t *testing.T) {
	engine := NewEngine()
	
	// Test that the sandboxing prevents actual system access
	t.Run("NoFileSystemAccess", func(t *testing.T) {
		// Rules should not be able to access the file system
		// Even if they contain file paths, no actual file access should occur
		rule := `when heap.alloc > 0 { log("/etc/passwd") }`
		err := engine.LoadRule("fs_test", rule)
		if err != nil {
			t.Errorf("Rule with file path should parse safely: %v", err)
		}
		
		// The rule should parse and evaluate without actually accessing files
		engine.Start()
		defer engine.Stop()
		
		// Let it run briefly
		time.Sleep(100 * time.Millisecond)
		
		// Engine should still be running (no crashes from file access attempts)
		if !engine.IsRunning() {
			t.Error("Engine should still be running after evaluating rules with file paths")
		}
	})
	
	t.Run("NoNetworkAccess", func(t *testing.T) {
		// Rules should not be able to make network requests
		rule := `when heap.alloc > 0 { alert("http://malicious.com/steal-data") }`
		err := engine.LoadRule("network_test", rule)
		if err != nil {
			t.Errorf("Rule with URL should parse safely: %v", err)
		}
		
		engine.Start()
		defer engine.Stop()
		
		time.Sleep(100 * time.Millisecond)
		
		if !engine.IsRunning() {
			t.Error("Engine should still be running after evaluating rules with URLs")
		}
	})
}

func TestInputSanitization(t *testing.T) {
	engine := NewEngine()
	
	// Test various forms of malicious input
	maliciousInputs := []struct {
		name  string
		input string
	}{
		{"Unicode Injection", "when heap.alloc > 0 { log(\"\u202e\u0645\u0644\u0641 \u202d\") }"},
		{"Control Characters", "when heap.alloc > 0 { log(\"\x1b[31mRed Text\x1b[0m\") }"},
		{"Long Unicode", "when heap.alloc > 0 { log(\"" + strings.Repeat("🔥", 1000) + "\") }"},
		{"Mixed Encoding", "when heap.alloc > 0 { log(\"\\xff\\xfe\\x41\\x00\") }"},
	}
	
	for _, tc := range maliciousInputs {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.LoadRule("sanitization_test", tc.input)
			// These should either parse safely or fail gracefully
			if err != nil && !strings.Contains(err.Error(), "parse errors") {
				t.Errorf("Unexpected error type for malicious input: %v", err)
			}
			
			// Clear for next test
			engine.ClearRules()
		})
	}
}

// Import required for test functions
import (
	"fmt"
	"time"
)