# Security Standards Quick Reference

**When to load:** Working on authentication, authorization, RBAC, or handling sensitive data

## Critical Security Rules

### Token Handling

**1. User Token Authentication Required**

```go
reqK8s, reqDyn := GetK8sClientsForRequest(c)
if reqK8s == nil {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
    c.Abort()
    return
}
```

**2. Token Redaction in Logs**

**FORBIDDEN:**

```go
log.Printf("Authorization: Bearer %s", token)
log.Printf("Request headers: %v", headers)
```

**REQUIRED:**

```go
log.Printf("Token length: %d", len(token))
path = strings.Split(path, "?")[0] + "?token=[REDACTED]"
```

**Token Redaction Pattern:** See `server/server.go:22-34`

```go
func customRedactingFormatter(param gin.LogFormatterParams) string {
    path := param.Path
    if strings.Contains(path, "token=") {
        path = strings.Split(path, "?")[0] + "?token=[REDACTED]"
    }
}
```

### RBAC Enforcement

**1. Always Check Permissions Before Operations**

```go
ssar := &authv1.SelfSubjectAccessReview{
    Spec: authv1.SelfSubjectAccessReviewSpec{
        ResourceAttributes: &authv1.ResourceAttributes{
            Group:     "vteam.ambient-code",
            Resource:  "agenticsessions",
            Verb:      "list",
            Namespace: project,
        },
    },
}
res, err := reqK8s.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, ssar, v1.CreateOptions{})
if err != nil || !res.Status.Allowed {
    c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
    return
}
```

**2. Namespace Isolation**

- Each project maps to a Kubernetes namespace
- User token must have permissions in that namespace
- Never bypass namespace checks

### Container Security

**Always Set SecurityContext for Job Pods**

```go
SecurityContext: &corev1.SecurityContext{
    AllowPrivilegeEscalation: boolPtr(false),
    ReadOnlyRootFilesystem:   boolPtr(false),  // Only if temp files needed
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},
    },
},
```

### Input Validation

**1. Validate All User Input**

```go
if !isValidK8sName(name) {
    c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid name format"})
    return
}
if _, err := url.Parse(repoURL); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository URL"})
    return
}
```

**2. Sanitize for Log Injection**

```go
name = strings.ReplaceAll(name, "\n", "")
name = strings.ReplaceAll(name, "\r", "")
```

## Exception: Public API Gateway

The `components/public-api/` service is intentionally different from the backend:

- **No K8s Clients**: Does NOT use `GetK8sClientsForRequest()` or access Kubernetes directly
- **No RBAC Permissions**: ServiceAccount has NO RoleBindings
- **Token Forwarding Only**: Proxies requests to backend with user's token
- **Backend Validates**: All K8s operations and RBAC enforcement happen in the backend

This separation minimizes the attack surface of the externally-exposed service.

## Common Security Patterns

### Pattern 1: Extracting Bearer Token

```go
rawAuth := c.GetHeader("Authorization")
parts := strings.SplitN(rawAuth, " ", 2)
if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
    return
}
token := strings.TrimSpace(parts[1])
log.Printf("Processing request with token (len=%d)", len(token))
```

### Pattern 2: Validating Project Access

```go
func ValidateProjectContext() gin.HandlerFunc {
    return func(c *gin.Context) {
        projectName := c.Param("projectName")
        reqK8s, _ := GetK8sClientsForRequest(c)
        if reqK8s == nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }
        // Check if user can access namespace via SelfSubjectAccessReview
        // ...
        c.Set("project", projectName)
        c.Next()
    }
}
```

### Pattern 3: Minting Service Account Tokens

```go
tokenRequest := &authv1.TokenRequest{
    Spec: authv1.TokenRequestSpec{
        ExpirationSeconds: int64Ptr(3600),
    },
}
tokenResponse, err := K8sClient.CoreV1().ServiceAccounts(namespace).CreateToken(
    ctx, serviceAccountName, tokenRequest, v1.CreateOptions{},
)
// Store token in secret (never log it)
```

## Security Checklist

**Authentication:**

- [ ] Using user token (GetK8sClientsForRequest) for user operations
- [ ] Returning 401 if token is invalid/missing
- [ ] Not falling back to service account on auth failure

**Authorization:**

- [ ] RBAC check performed before resource access
- [ ] Using correct namespace for permission check
- [ ] Returning 403 if user lacks permissions

**Secrets & Tokens:**

- [ ] No tokens in logs (use len(token) instead)
- [ ] No tokens in error messages
- [ ] Tokens stored in Kubernetes Secrets
- [ ] Token redaction in request logs

**Input Validation:**

- [ ] All user input validated
- [ ] Resource names validated (K8s DNS label format)
- [ ] URLs parsed and validated
- [ ] Log injection prevented

**Container Security:**

- [ ] SecurityContext set on all Job pods
- [ ] AllowPrivilegeEscalation: false
- [ ] Capabilities dropped (ALL)
- [ ] OwnerReferences set for cleanup

## Production Security

- **API keys**: Store in Kubernetes Secrets, managed via ProjectSettings CR
- **RBAC**: Namespace-scoped isolation prevents cross-project access
- **OAuth integration**: OpenShift OAuth for cluster-based authentication (see `docs/deployment/OPENSHIFT_OAUTH.md`)
- **Network policies**: Component isolation and secure communication
