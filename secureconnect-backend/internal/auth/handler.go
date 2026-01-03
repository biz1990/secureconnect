package auth

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    
    "secureconnect-backend/internal/database"
    "secureconnect-backend/internal/models"
    "secureconnect-backend/pkg/logger"
)

type AuthHandler struct {
    DB *database.DB
}

// NewAuthHandler khởi tạo handler với DB dependency
func NewAuthHandler(db *database.DB) *AuthHandler {
    return &AuthHandler{DB: db}
}

func (h *AuthHandler) Register(c *gin.Context) {
    var user models.User
    if err := c.ShouldBindJSON(&user); err != nil {
        logger.Error("Payload không hợp lệ", err.Error())
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
        return
    }

    // 1. Mã hóa mật khẩu (Hashing)
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    if err != nil {
        logger.Error("Lỗi băm mật khẩu", err.Error())
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

    // 2. Gán ID và thời gian
    user.UserID = uuid.New()
    user.CreatedAt = time.Now()

    // 3. Lưu vào Database
    // Sử dụng `stdsql` cho câu lệnh SQL đơn giản
    _, err = h.DB.GetPool().Exec(c, 
        `INSERT INTO users (user_id, email, password, full_name, created_at) 
        VALUES ($1, $2, $3, $4, $5)`,
        user.UserID, user.Email, string(hashedPassword), user.FullName, user.CreatedAt,
    )

    if err != nil {
        logger.Error("Lỗi lưu user vào DB", err.Error())
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user"})
        return
    }

    logger.Info("Đăng ký thành công", user.Email)
    c.JSON(http.StatusCreated, gin.H{
        "success": true,
        "user_id": user.UserID,
        "email": user.Email,
    })
}

func (h *AuthHandler) Login(c *gin.Context) {
    var req struct {
        Email    string `json:"email" binding:"required"`
        Password string `json:"password" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
        return
    }

    // 1. Query User từ DB
    var user models.User
    err := h.DB.GetPool().QueryRow(c, 
        "SELECT user_id, email, password, full_name, created_at FROM users WHERE email = $1", 
        req.Email,
    ).Scan(&user.UserID, &user.Email, &user.Password, &user.FullName, &user.CreatedAt)

    if err != nil {
        logger.Error("Không tìm thấy user", req.Email)
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
        return
    }

    // 2. Kiểm tra mật khẩu (Verify Hash)
    err = bcrypt.CompareHashAndPassword([]byte(req.Password), []byte(user.Password))
    if err != nil {
        logger.Error("Sai mật khẩu", req.Email)
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
        return
    }

    // 3. Tạo JWT Token (Giả lập)
    // Trong thực tế, bạn cần cấu hình Claims chứa user_id, expire time...
    expirationTime := time.Now().Add(24 * time.Hour)
    claims := jwt.MapClaims{
        "sub": user.UserID,
        "exp": expirationTime.Unix(),
        "iat": time.Now().Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte("super-secret-key")) // Trong production dùng h.Secret
    if err != nil {
        logger.Error("Lỗi tạo token", err.Error())
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "token": tokenString,
        "user": gin.H{
            "user_id": user.UserID,
            "email":   user.Email,
        },
    })
}

// package auth

// import (
//     "net/http"

//     "github.com/gin-gonic/gin"
//     "github.com/google/uuid"
//     "golang.org/x/crypto/bcrypt"
    
//     "secureconnect-backend/internal/database"
//     "secureconnect-backend/internal/models"
//     "secureconnect-backend/pkg/logger"
// )

// type AuthHandler struct {
//     DB *database.DB
// }

// func NewAuthHandler(db *database.DB) *AuthHandler {
//     return &AuthHandler{DB: db}
// }

// func (h *AuthHandler) Register(c *gin.Context) {
//     var user models.User
//     if err := c.ShouldBindJSON(&user); err != nil {
//         logger.Error("Invalid payload", err.Error())
//         c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//         return
//     }

//     // Hash password (Giả lập)
//     hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
//     if err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
//         return
//     }
//     user.Password = string(hashedPassword)
//     user.UserID = uuid.New()

//     // Save to DB (Giả lập logic insert)
//     _, err = h.DB.Pool.Exec(c, "INSERT INTO users (user_id, email, password, full_name, created_at) VALUES ($1, $2, $3, $4, NOW())",
//         user.UserID, user.Email, user.Password, user.FullName)
//     if err != nil {
//         logger.Error("Failed to insert user", err.Error())
//         c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user"})
//         return
//     }

//     c.JSON(http.StatusCreated, gin.H{
//         "success": true,
//         "user_id": user.UserID,
//     })
// }

// func (h *AuthHandler) Login(c *gin.Context) {
//     // Logic Login tương tự
//     logger.Info("Login called")
//     c.JSON(http.StatusOK, gin.H{"token": "fake-jwt-token"})
// }