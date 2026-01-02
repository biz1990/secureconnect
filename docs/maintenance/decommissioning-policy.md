

Dưới đây là nội dung chi tiết cho file **`maintenance/decommissioning-policy.md`**. Tài liệu này quy định quy trình chuẩn mực để loại bỏ (decommission) các phiên bản phần mềm cũ, API đã lỗi thời hoặc hạ tầng không còn sử dụng, đảm bảo sự chuyển đổi êm ái cho người dùng cuối.

---

# Maintenance & Decommissioning Policy

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 24.1. Tổng quan

Mục tiêu của chính sách này là quản lý vòng đời (Lifecycle) của các thành phần hệ thống, đảm bảo:
1.  **Bảo mật:** Loại bỏ các phiên bản có lỗ hổng đã biết.
2.  **Hiệu năng:** Không chịu gánh nặng bảo trì các hệ thống lỗi thời (Legacy).
3.  **Trải nghiệm người dùng:** Đảm bảo người dùng có đủ thời gian để cập nhật mà không bị ngắt quãng dịch vụ.

### Nguyên tắc cốt lõi
*   **Quy tắc "N - 1":** Khi phiên bản mới (N) ra mắt, phiên bản N-2 sẽ được đánh dấu là "Sắp ngừng hỗ trợ" (Deprecated).
*   **Thời gian thông báo:** Tối thiểu **90 ngày** thông báo trước khi ngừng hỗ trợ chính thức một tính năng hoặc phiên bản.

---

## 24.2. Vòng đời Phiên bản (Version Lifecycle)

Chúng ta áp dụng quy tắc quản lý phiên bản **Semantic Versioning** (Major.Minor.Patch).

### Các giai đoạn

| Giai đoạn | Định nghĩa | Hành động kỹ thuật | Giao diện người dùng |
| :--- | :--- | :--- | :--- |
| **Active** | Phiên bản hiện tại, nhận các bản cập nhật bảo mật và tính năng. | Viết code, fix bugs, deploy. | Giao diện mới nhất, khuyến khích sử dụng. |
| **Maintenance** | Phiên bản ổn định, chỉ nhận bản vá lỗi (Patches), không thêm tính năng mới. | Chỉ merge Hotfixes. | Vẫn ổn định, được hỗ trợ đầy đủ. |
| **Deprecated** | Phiên bản cũ, sắp ngừng hỗ trợ. | Dừng phát triển tính năng mới, chuẩn bị code để tắt. | Cảnh báo trong App/Website: "Vui lòng cập nhật để trải nghiệm tốt nhất". |
| **End of Support (EOS)** | Ngừng hỗ trợ hoàn toàn. | Tắt server endpoints, xóa code legacy, tắt API. | Chặn truy cập, yêu cầu cập nhật bắt buộc. |

**Ví dụ thực tế:**
*   SecureConnect **v1.0** (Release 01/2023).
*   SecureConnect **v1.1** (Release 06/2023).
*   SecureConnect **v2.0** (Release 01/2024) -> Phiên bản v1.0 trở thành **Deprecated**.

---

## 24.3. Chính sách API Deprecation (Ngừng API)

Dùng cho REST API và WebSocket.

### Quy trình 4 bước:

#### Bước 1: Thông báo Deprecation (Day 1)
*   Trong swagger/OpenAPI, đánh dấu endpoint cũ: `deprecated: true`.
*   Thêm Header vào Response của endpoint đó:
    ```http
    HTTP/1.1 200 OK
    X-API-Deprecation: true
    X-API-Sunset-Date: 2024-04-01T00:00:00Z
    Link: <https://api.secureconnect.com/v2/new-endpoint>; rel="successor-version"
    ```
*   Client (Flutter) nên kiểm tra header này để hiện thông báo "API này sắp hết hạn".

#### Bước 2: Giai đoạn chuyển tiếp (Transition Period)
*   Vẫn giữ endpoint cũ hoạt động.
*   Ghi log cảnh báo trong Backend: `logger.Warn("Deprecated API called by client version x.y.z")` để theo dõi lượng người dùng còn dùng version cũ.

#### Bước 3: Sunset (Ngừng hoạt động)
*   Sau ngày `Sunset Date`, Endpoint cũ trả về mã trạng thái `410 Gone`.
*   Response Body:
    ```json
    {
      "success": false,
      "error": {
        "code": "API_DEPRECATED",
        "message": "This endpoint is no longer supported. Please update to v2.0",
        "docs_url": "https://docs.secureconnect.com/migration-guide"
      }
    }
    ```

#### Bước 4: Xóa code
*   Xóa endpoint khỏi code Backend (Go).
*   Xóa khỏi Swagger docs.

**Quy tắc:** Tối thiểu giữ thời gian chuyển tiếp là **6 tháng** kể từ ngày thông báo Deprecation.

---

## 24.4. Chính sách Ứng dụng Client (Flutter) Deprecation

Quản lý phiên bản ứng dụng trên Mobile (App Store/Play Store) và Web.

### 1. Cập nhật bắt buộc (Force Update)

Khi phát hiện một phiên bản quá cũ và có lỗ hổng nghiêm trọng (Critical Security):

**Logic Client (Flutter):**
```dart
Future<void> checkVersion() async {
  final serverMinVersion = await apiService.getMinSupportedVersion(); // Lấy từ config server
  final currentVersion = await getAppVersion();

  if (currentVersion < serverMinVersion) {
    // Hiển thị Dialog không thể đóng (Block UI)
    showForceUpdateDialog(
      title: "Cập nhật bắt buộc",
      message: "Phiên bản bạn đang dùng đã quá cũ và có rủi ro bảo mật. Vui lòng cập nhật ngay.",
      onUpdate: () => openStore(AppStore),
    );
  }
}
```

**Logic Server (Go API):**
```go
// Middleware kiểm tra version Client
func VersionCheckMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        clientVersion := c.GetHeader("X-App-Version")
        minSupportedVersion := "1.5.0"

        if compareVersions(clientVersion, minSupportedVersion) < 0 {
            c.JSON(426, gin.H{ // 426 Upgrade Required
                "error": "UPDATE_REQUIRED",
                "min_version": minSupportedVersion,
                "download_url": "https://secureconnect.com/download"
            })
            c.Abort()
        }
        c.Next()
    }
}
```

### 2. Ngừng hỗ trợ hệ điều hành (OS Deprecation)

Để tối ưu hiệu năng và bảo mật, chúng ta sẽ ngừng hỗ trợ các OS cũ.

*   **iOS:** Tối thiểu hỗ trợ **iOS 14+** (thường là iOS chính 2 năm gần nhất). Người dùng iOS 13 sẽ không thể cài bản cập nhật mới.
*   **Android:** Tối thiểu hỗ trợ **Android 8.0 (API 26)+**.
*   **Flutter Web:** Tối thiểu hỗ trợ 2 trình duyệt mới nhất: Chrome, Edge, Firefox, Safari.

---

## 24.5. Chính sách Infrastructure Decommissioning (K8s & DB)

Dùng khi nâng cấp hạ tầng (ví dụ: đổi Node version, nâng DB engine).

### 1. Kubernetes Cluster
*   **Node Rotation:** Không bao giờ tắt toàn bộ cluster một lúc.
*   **Drain & Cordon:** Dùng `kubectl cordon` để đánh dấu node không nhận pod mới, rồi `kubectl drain` để di chuyển pod sang node khác, rồi mới tắt node.
*   **Phiên bản K8s:** Hỗ trợ phiên bản N và N-1 (theo chính sách Kubernetes). K8s N-2 không được deploy mới.

### 2. Database (Cassandra/CockroachDB)
Việc nâng cấp DB rất rủi ro.

*   **Blue-Green Migration:**
    1.  Deploy bản DB mới lên một Cluster riêng (Green).
    2.  Sử dụng công cụ Replication để đồng bộ data từ Cluster cũ (Blue) sang mới (Green).
    3.  Đảo lưu traffic của một số service sang Cluster mới.
    4.  Quan sát trong 1 tuần. Nếu ổn định, chuyển toàn bộ traffic.
    5.  Giữ lại Cluster cũ (Blue) chạy thêm **1 tháng** để làm Rollback nếu cần.
    6.  Sau 1 tháng, Backup lại (dump) và tắt Cluster cũ.

---

## 24.6. Chính sách WebRTC Protocol Deprecation

Đảm bảo tương thích ngược cho Video Call giữa các phiên bản Client khác nhau.

*   **Codec:**
    *   Khi thêm codec mới (ví dụ: AV1), Server phải tiếp tục hỗ trợ VP8/H264 ít nhất 6 tháng.
    *   Thông báo qua SDP: `a=fmtp:VP8 deprecated-date=2024-06-01`.
*   **Signaling Protocol:**
    *   Nếu thay đổi format JSON Signaling, Server phải hỗ trợ cả 2 format (Parser V1 và Parser V2) trong một thời gian.
    *   Client gửi header `X-Client-Signaling-Version: 1`.
    *   Server chuyển đổi response sang format tương ứng.

---

## 24.7. Xử lý Dữ liệu khi Decommission (Data Handling)

Khi ngừng một tính năng, ta làm gì với dữ liệu đã tạo?

*   **Chính sách "Lưu trữ lạnh" (Cold Storage):**
    *   Khi tắt một tính năng chat cũ, không xóa DB ngay lập tức.
    *   Export dữ liệu ra **S3 Glacier** (lưu trữ giá rẻ) để giữ tuân thủ pháp lý (đôi khi cần forensic).
*   **Anonymization:** Sau **5 năm**, nếu không còn nhu cầu pháp lý, thực hiện xóa vĩnh viễn hoặc Anonymize dữ liệu (xóa các trường `email`, `phone`, giữ lại `user_id` giả lập).
*   **Xóa file vật lý:** Xóa các file ảnh/video trên MinIO đã hết hạn retention (TTL) hoặc liên quan đến user đã xóa tài khoản.

---

## 24.8. Kế hoạch Truyền thông (Communication Plan)

Để tránh phản ứng tiêu cực từ người dùng, truyền thông là quan trọng nhất.

1.  **Trong App (In-App Notification):**
    *   Ngày -90: "Phiên bản v1.0 sẽ ngừng hỗ trợ vào ngày 01/01/2024. Chúng tôi khuyến khích bạn cập nhật v2.0 ngay."
    *   Ngày -30: "Chỉ còn 30 ngày để cập nhật. Sau ngày này ứng dụng sẽ không hoạt động."
    *   Ngày -7: "Cảnh báo cuối cùng."

2.  **Email Marketing:**
    *   Gửi email cho các user chưa cập nhật vào ngày -30 và -7.

3.  **Blog/Changelog:**
    *   Viết bài "Tại sao chúng ta ngừng hỗ trợ Android 7?" để giải thích lý do bảo mật, hiệu năng.

---

## 24.9. Quy trình Rollback (Khẩn cấp)

Quy định: **Chỉ được xóa/tắt hoàn toàn hệ thống cũ sau khi hệ thống mới chạy ổn định ít nhất 2 tuần (14 ngày) không lỗi nghiêm trọng (P0/P1).**

*   Nếu hệ thống mới gặp bug Critical (P0):
    1.  Kích hoạt lại ngay lập tức hệ thống cũ (Ví dụ: Redis cũ, Database cũ, App version cũ trên store nếu chưa bị thu hồi).
    2.  Thông báo cho người dùng: "Chúng tôi đang gặp lỗi trên phiên bản mới, hệ thống đã tự động đưa bạn về phiên bản ổn định."

---

## 24.10. Checklist Decommissioning (Dành cho Kỹ sư)

Khi chuẩn bị ngừng một thành phần (ví dụ: `v1/auth/login`), checklist này phải được điền đầy đủ:

*   **Xác định:** Endpoint/Service/Component nào?
*   **Người dùng bị ảnh hưởng:** Bao nhiêu user vẫn đang dùng? (Kiểm tra logs)
*   **Ngày Deprecate:** Ngày thông báo đầu tiên.
*   **Ngày Sunset:** Ngày chính thức tắt (HTTP 410).
*   **Cơ chế Migration:** Đã viết hướng dẫn migration cho user chưa?
*   **Code Backup:** Đã tag code branch (ví dụ: `git tag backup-before-deprecate`) chưa?
*   **Fallback Plan:** Nếu tắt xong xảy ra lỗi, cách bật lại nhanh nhất là gì?
*   **Communication:** Đã có thông báo trên Web/Mobile/Email chưa?

---

*Kết thúc danh sách tài liệu kiến trúc.*