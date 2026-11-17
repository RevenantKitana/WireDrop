package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// PayloadType xác định loại vật phẩm đang nằm trong đường ống
type PayloadType int

const (
	PayloadEmpty       PayloadType = iota // Đường ống đang rỗng
	PayloadText                           // Đang chứa 1 đoạn Lệnh / Script / Text
	PayloadFile                           // Đang chứa 1 File duy nhất
	PayloadChunkStream                    // Đang trong phiên truyền file lớn cắt mảnh 16MB
)

const (
	ChunkSize       = 16 * 1024 * 1024 // 16MB chuẩn nhị phân
	ChunkIdleHold   = 10 * time.Minute // Thời gian giữ Chunk 0 tối đa nếu RX chưa kết nối
	ChunkStallLimit = 45 * time.Second // Thời gian chờ tối đa khi truyền 1 chunk
)

// ChunkItem đại diện cho 1 mảnh 16MB đang nằm trong bộ đệm RAM
type ChunkItem struct {
	Index    int
	Data     []byte
	Size     int
	Checksum string
}

// ChunkSession quản lý luân chuyển các chunk 16MB theo cơ chế Ping-Pong Buffer (Dung lượng đệm = 2 slots)
type ChunkSession struct {
	sync.Mutex
	SessionID   string
	FileName    string
	TotalSize   int64
	TotalChunks int
	CreatedAt   time.Time
	LastActive  time.Time

	// Buffer channel chứa tối đa 2 chunks (32MB RAM cap)
	chunkChan chan *ChunkItem

	// Kênh đồng bộ và ngắt
	doneChan  chan struct{}
	cancelled bool
	cancelMsg string

	// Timer tự hủy Chunk 0 nếu RX không vào sau 10 phút
	idleTimer *time.Timer
}

func NewChunkSession(sessionID, fileName string, totalSize int64, totalChunks int) *ChunkSession {
	return &ChunkSession{
		SessionID:   sessionID,
		FileName:    fileName,
		TotalSize:   totalSize,
		TotalChunks: totalChunks,
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		chunkChan:   make(chan *ChunkItem, 2), // Ping-Pong Buffer (Window = 2)
		doneChan:    make(chan struct{}),
	}
}

// PushChunk đẩy một chunk vào buffer (chờ tối đa timeout nếu buffer 2 slots đang đầy)
func (s *ChunkSession) PushChunk(item *ChunkItem, timeout time.Duration) (bool, string) {
	s.Lock()
	if s.cancelled {
		msg := s.cancelMsg
		s.Unlock()
		return false, msg
	}
	s.LastActive = time.Now()
	s.Unlock()

	select {
	case s.chunkChan <- item:
		return true, ""
	case <-s.doneChan:
		s.Lock()
		defer s.Unlock()
		return false, s.cancelMsg
	case <-time.After(timeout):
		return false, "Push chunk timeout (45s stall)"
	}
}

// PopChunk lấy một chunk từ buffer xả sang RX (chờ tối đa timeout)
func (s *ChunkSession) PopChunk(timeout time.Duration) (*ChunkItem, bool, string) {
	select {
	case item, ok := <-s.chunkChan:
		if !ok {
			return nil, false, "Session closed"
		}
		s.Lock()
		s.LastActive = time.Now()
		s.Unlock()
		return item, true, ""
	case <-s.doneChan:
		s.Lock()
		defer s.Unlock()
		return nil, false, s.cancelMsg
	case <-time.After(timeout):
		return nil, false, "Pop chunk timeout (45s stall)"
	}
}

// Cancel hủy phiên truyền file do một trong hai bên ngắt hoặc quá hạn
func (s *ChunkSession) Cancel(reason string) {
	s.Lock()
	if s.cancelled {
		s.Unlock()
		return
	}
	s.cancelled = true
	s.cancelMsg = reason
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	close(s.doneChan)
	s.Unlock()
}

// PipeSlot là khe cắm nguyên tử duy nhất của phòng (chỉ chứa 1 vật phẩm tại 1 thời điểm)
type PipeSlot struct {
	Type      PayloadType
	TextData  string
	FileName  string
	FileData  []byte // Dùng cho file <= 15MB lưu trong RAM
	TempPath  string // Dùng cho file > 15MB lưu trên đĩa tạm
	FileSize  int64
	CreatedAt time.Time
}

// ActiveConn đại diện cho 1 kết nối vật lý đang cắm vào một đầu dây
type ActiveConn struct {
	ID          string    // Mã định danh duy nhất của phiên kết nối (conn_id)
	ConnectedAt time.Time // Thời điểm kết nối bắt đầu
}

// RoomState đại diện cho một sợi dây 1 chiều nối từ Sender sang Receiver
type RoomState struct {
	sync.RWMutex
	Slug string

	// Khe cắm dữ liệu nguyên tử (Single Slot)
	Slot PipeSlot

	// Cơ chế Khóa Kết Nối Vật Lý 1-1 Thuần Túy (Pure Physical Connection Lock)
	SenderConn   *ActiveConn // Kết nối duy nhất đang giữ đầu phát (*-s)
	ReceiverConn *ActiveConn // Kết nối duy nhất đang giữ đầu nhận (*-r)

	BurnAfterRead bool // Tự hủy dữ liệu ngay sau khi Máy B rút ra 1 lần
	LastActive    time.Time

	// Subscribers SSE cập nhật trạng thái cho Web UI
	sseSubscribers map[chan string]bool

	// Dành cho Live Terminal Pipe (-s / -r streaming liên tục)
	streamSubscribers map[chan string]bool

	// Dành cho Zero-Storage Pass-Through Pipe (File cực lớn)
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	pipeLock   sync.Mutex

	// Dành cho cơ chế 16MB Chunked Dual-Buffering Pipe
	ChunkSess *ChunkSession
}

// RoomManager quản lý danh sách các sợi dây phòng trong RAM
type RoomManager struct {
	sync.RWMutex
	rooms map[string]*RoomState
}

var globalRooms = &RoomManager{
	rooms: make(map[string]*RoomState),
}

// NewRoom khởi tạo một sợi dây phòng mới
func NewRoom(slug string) *RoomState {
	return &RoomState{
		Slug: slug,
		Slot: PipeSlot{
			Type:      PayloadEmpty,
			CreatedAt: time.Now(),
		},
		BurnAfterRead:     true, // Mặc định tự hủy sau khi nhận để an toàn bảo mật
		sseSubscribers:    make(map[chan string]bool),
		streamSubscribers: make(map[chan string]bool),
		LastActive:        time.Now(),
	}
}

// GetOrCreate lấy hoặc tạo mới phòng theo slug cơ sở (ví dụ: "1")
func (m *RoomManager) GetOrCreate(slug string) *RoomState {
	m.Lock()
	defer m.Unlock()

	room, exists := m.rooms[slug]
	if !exists {
		room = NewRoom(slug)
		m.rooms[slug] = room
	}
	room.Touch()
	return room
}

// Get tìm phòng đang tồn tại
func (m *RoomManager) Get(slug string) (*RoomState, bool) {
	m.RLock()
	defer m.RUnlock()

	room, exists := m.rooms[slug]
	return room, exists
}

// Delete xóa phòng và giải phóng tài nguyên
func (m *RoomManager) Delete(slug string) {
	m.Lock()
	room, exists := m.rooms[slug]
	if exists {
		delete(m.rooms, slug)
	}
	m.Unlock()

	if exists && room != nil {
		room.Cleanup()
	}
}

// StartTTLWorker định kỳ dọn dẹp các phòng quá hạn (2 giờ không hoạt động)
func (m *RoomManager) StartTTLWorker(checkInterval time.Duration, maxIdle time.Duration) {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			var expiredSlugs []string

			m.RLock()
			for slug, room := range m.rooms {
				room.RLock()
				if now.Sub(room.LastActive) > maxIdle {
					expiredSlugs = append(expiredSlugs, slug)
				}
				room.RUnlock()
			}
			m.RUnlock()

			for _, slug := range expiredSlugs {
				m.Delete(slug)
			}
		}
	}()
}

// Touch cập nhật thời gian hoạt động gần nhất
func (r *RoomState) Touch() {
	r.LastActive = time.Now()
}

// Cleanup dọn dẹp file tạm và đóng các channel
func (r *RoomState) Cleanup() {
	r.Lock()
	defer r.Unlock()

	if r.ChunkSess != nil {
		r.ChunkSess.Cancel("Room cleanup")
		r.ChunkSess = nil
	}

	if r.Slot.TempPath != "" {
		_ = os.Remove(r.Slot.TempPath)
	}
	r.Slot = PipeSlot{Type: PayloadEmpty}

	for ch := range r.sseSubscribers {
		close(ch)
	}
	r.sseSubscribers = make(map[chan string]bool)

	for ch := range r.streamSubscribers {
		close(ch)
	}
	r.streamSubscribers = make(map[chan string]bool)

	if r.pipeWriter != nil {
		_ = r.pipeWriter.Close()
	}
}

// SetText nạp một đoạn Text/Script vào đường ống (thay thế vật phẩm cũ)
func (r *RoomState) SetText(text string) {
	r.Lock()
	defer r.Unlock()

	if r.ChunkSess != nil {
		r.ChunkSess.Cancel("Replaced by text")
		r.ChunkSess = nil
	}

	// Dọn dẹp file tạm cũ nếu có
	if r.Slot.TempPath != "" {
		_ = os.Remove(r.Slot.TempPath)
	}

	r.Slot = PipeSlot{
		Type:      PayloadText,
		TextData:  text,
		CreatedAt: time.Now(),
	}
	r.Touch()
}

// SetFile nạp một File nhị phân vào đường ống (thay thế vật phẩm cũ)
func (r *RoomState) SetFile(name string, size int64, data []byte, tempPath string) {
	r.Lock()
	defer r.Unlock()

	if r.ChunkSess != nil {
		r.ChunkSess.Cancel("Replaced by single file")
		r.ChunkSess = nil
	}

	// Dọn dẹp file tạm cũ nếu có
	if r.Slot.TempPath != "" && r.Slot.TempPath != tempPath {
		_ = os.Remove(r.Slot.TempPath)
	}

	r.Slot = PipeSlot{
		Type:      PayloadFile,
		FileName:  name,
		FileSize:  size,
		FileData:  data,
		TempPath:  tempPath,
		CreatedAt: time.Now(),
	}
	r.Touch()
}

// StartChunkSession khởi tạo phiên truyền file cắt mảnh 16MB mới
func (r *RoomState) StartChunkSession(sessionID, fileName string, totalSize int64, totalChunks int) *ChunkSession {
	r.Lock()
	defer r.Unlock()

	if r.ChunkSess != nil {
		r.ChunkSess.Cancel("Superseded by new session")
		r.ChunkSess = nil
	}

	if r.Slot.TempPath != "" {
		_ = os.Remove(r.Slot.TempPath)
	}

	sess := NewChunkSession(sessionID, fileName, totalSize, totalChunks)
	r.ChunkSess = sess
	r.Slot = PipeSlot{
		Type:      PayloadChunkStream,
		FileName:  fileName,
		FileSize:  totalSize,
		CreatedAt: time.Now(),
	}
	r.Touch()
	return sess
}

// GetChunkSession lấy session truyền file cắt mảnh hiện tại (nếu có)
func (r *RoomState) GetChunkSession() *ChunkSession {
	r.RLock()
	defer r.RUnlock()
	return r.ChunkSess
}

// ClearSlot rút cạn đường ống về trạng thái Rỗng
func (r *RoomState) ClearSlot() {
	r.Lock()
	defer r.Unlock()

	if r.ChunkSess != nil {
		r.ChunkSess.Cancel("Slot cleared")
		r.ChunkSess = nil
	}

	if r.Slot.TempPath != "" {
		_ = os.Remove(r.Slot.TempPath)
	}
	r.Slot = PipeSlot{Type: PayloadEmpty}
	r.Touch()
}

// =========================================================================
// QUẢN LÝ KHÓA KẾT NỐI VẬT LÝ (PHYSICAL CONNECTION LOCK)
// =========================================================================

// AcquireSender cố gắng chiếm giữ đầu phát (*-s). Hỗ trợ cờ force để cướp quyền nếu tab cũ bị treo.
func (r *RoomState) AcquireSender(connID string, force bool) (bool, string) {
	r.Lock()
	defer r.Unlock()

	if r.SenderConn == nil || r.SenderConn.ID == connID || force {
		r.SenderConn = &ActiveConn{
			ID:          connID,
			ConnectedAt: time.Now(),
		}
		r.broadcastSSEInternal("sender_paired", connID)
		return true, ""
	}

	return false, "Sender end (*-s) is already held by another active window/connection. Line busy."
}

// ReleaseSender giải phóng đầu phát (*-s) khi kết nối đóng
func (r *RoomState) ReleaseSender(connID string) {
	r.Lock()
	defer r.Unlock()

	if r.SenderConn != nil && r.SenderConn.ID == connID {
		r.SenderConn = nil
		r.broadcastSSEInternal("sender_unpaired", "")
	}
}

// CheckSenderAuth kiểm tra quyền gửi dữ liệu vào đầu phát (*-s)
func (r *RoomState) CheckSenderAuth(connID string) (bool, string) {
	r.Lock()
	defer r.Unlock()

	if connID != "" && r.SenderConn != nil {
		r.SenderConn.ID = connID
	}
	return true, ""
}

// AcquireReceiver cố gắng chiếm giữ đầu nhận (*-r). Hỗ trợ cờ force để cướp quyền nếu tab cũ bị treo.
func (r *RoomState) AcquireReceiver(connID string, force bool) (bool, string) {
	r.Lock()
	defer r.Unlock()

	if r.ReceiverConn == nil || r.ReceiverConn.ID == connID || force {
		r.ReceiverConn = &ActiveConn{
			ID:          connID,
			ConnectedAt: time.Now(),
		}
		r.broadcastSSEInternal("partner_paired", connID)
		return true, ""
	}

	return false, "Receiver end (*-r) is already held by another active window/connection. Line busy."
}

// ReleaseReceiver giải phóng đầu nhận (*-r) khi kết nối đóng
func (r *RoomState) ReleaseReceiver(connID string) {
	r.Lock()
	defer r.Unlock()

	if r.ReceiverConn != nil && r.ReceiverConn.ID == connID {
		r.ReceiverConn = nil
		r.broadcastSSEInternal("partner_unpaired", "")
	}
}

// ResetReceiver cưỡng chế giải phóng đầu nhận (nút Unpair trên UI bên phát)
func (r *RoomState) ResetReceiver() {
	r.Lock()
	defer r.Unlock()

	r.ReceiverConn = nil
	r.broadcastSSEInternal("partner_unpaired", "")
}

// CheckReceiverAuth kiểm tra quyền cắm vào đầu nhận (*-r) để lấy dữ liệu
func (r *RoomState) CheckReceiverAuth(connID string) (bool, string) {
	r.RLock()
	defer r.RUnlock()

	// Luôn cho phép lấy dữ liệu khi đọc một lần (CLI hoặc Web đã ghép nối)
	return true, ""
}

// =========================================================================
// REALTIME EVENTS & STREAMING
// =========================================================================

// SubscribeSSE đăng ký kênh cập nhật giao diện Web
func (r *RoomState) SubscribeSSE() chan string {
	r.Lock()
	defer r.Unlock()

	ch := make(chan string, 20)
	r.sseSubscribers[ch] = true
	return ch
}

// UnsubscribeSSE hủy đăng ký kênh SSE
func (r *RoomState) UnsubscribeSSE(ch chan string) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.sseSubscribers[ch]; ok {
		delete(r.sseSubscribers, ch)
		close(ch)
	}
}

// BroadcastSSE đẩy dữ liệu sự kiện tới tất cả Web UI đang mở
func (r *RoomState) BroadcastSSE(event string, data string) {
	r.RLock()
	defer r.RUnlock()
	r.broadcastSSEInternal(event, data)
}

func (r *RoomState) broadcastSSEInternal(event string, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, strings.ReplaceAll(data, "\n", "\\n"))
	for ch := range r.sseSubscribers {
		select {
		case ch <- msg:
		default:
		}
	}
}

// SubscribeStream đăng ký nhận lệnh Live Terminal Pipe
func (r *RoomState) SubscribeStream() chan string {
	r.Lock()
	defer r.Unlock()

	ch := make(chan string, 10)
	r.streamSubscribers[ch] = true
	return ch
}

// UnsubscribeStream hủy đăng ký nhận lệnh Live Pipe
func (r *RoomState) UnsubscribeStream(ch chan string) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.streamSubscribers[ch]; ok {
		delete(r.streamSubscribers, ch)
		close(ch)
	}
}

// BroadcastCommand gửi lệnh tới bash của Máy B
func (r *RoomState) BroadcastCommand(cmd string) {
	r.RLock()
	defer r.RUnlock()

	for ch := range r.streamSubscribers {
		select {
		case ch <- cmd:
		default:
		}
	}
}
