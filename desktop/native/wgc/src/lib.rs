// Go-owned native WGC bridge. Frames stay in memory and bind an exact HWND.
use std::sync::{Arc, Mutex};
use windows_capture::capture::{CaptureControl, Context, GraphicsCaptureApiHandler};
use windows_capture::frame::Frame;
use windows_capture::graphics_capture_api::InternalCaptureControl;
use windows_capture::settings::{ColorFormat, CursorCaptureSettings, DirtyRegionSettings, DrawBorderSettings, MinimumUpdateIntervalSettings, SecondaryWindowSettings, Settings};
use windows_capture::window::Window;

type CaptureError = Box<dyn std::error::Error + Send + Sync>;

#[derive(Default)]
struct FrameState {
    bytes: Vec<u8>,
    width: u32,
    height: u32,
    sequence: u64,
    closed: bool,
}

struct CaptureState {
    frame: Arc<Mutex<FrameState>>,
    control: Option<CaptureControl<Handler, CaptureError>>,
}

struct Handler { frame: Arc<Mutex<FrameState>>, scratch: Vec<u8> }

impl GraphicsCaptureApiHandler for Handler {
    type Flags = Arc<Mutex<FrameState>>;
    type Error = CaptureError;
    fn new(ctx: Context<Self::Flags>) -> Result<Self, Self::Error> {
        Ok(Self { frame: ctx.flags, scratch: Vec::new() })
    }
    fn on_frame_arrived(&mut self, frame: &mut Frame, _: InternalCaptureControl) -> Result<(), Self::Error> {
        let width = frame.width();
        let height = frame.height();
        let buffer = frame.buffer()?;
        let bytes = buffer.as_nopadding_buffer(&mut self.scratch);
        // Black frames are not valid retries or first frames.
        if !bytes.chunks_exact(4).any(|pixel| pixel[0] != 0 || pixel[1] != 0 || pixel[2] != 0) { return Ok(()); }
        let mut state = self.frame.lock().map_err(|_| "frame lock failed")?;
        state.bytes.clear(); state.bytes.extend_from_slice(bytes);
        state.width = width; state.height = height; state.sequence += 1;
        Ok(())
    }
    fn on_closed(&mut self) -> Result<(), Self::Error> {
        self.frame.lock().map_err(|_| "frame lock failed")?.closed = true;
        Ok(())
    }
}

#[no_mangle]
pub extern "C" fn junshi_wgc_start(hwnd: usize) -> usize {
    if hwnd == 0 { return 0; }
    let frame = Arc::new(Mutex::new(FrameState::default()));
    let settings = Settings::new(Window::from_raw_hwnd(hwnd as *mut std::ffi::c_void),
        CursorCaptureSettings::Default, DrawBorderSettings::Default, SecondaryWindowSettings::Default,
        MinimumUpdateIntervalSettings::Default, DirtyRegionSettings::Default, ColorFormat::Rgba8, frame.clone());
    match Handler::start_free_threaded(settings) {
        Ok(control) => Box::into_raw(Box::new(CaptureState { frame, control: Some(control) })) as usize,
        Err(_) => 0,
    }
}

// Return 1 for a copied RGBA frame, 0 before first frame, 2 for insufficient
// capacity (dimensions still returned), and -1 for a closed/error session.
#[no_mangle]
pub unsafe extern "C" fn junshi_wgc_frame(handle: usize, output: *mut u8, capacity: usize,
    width: *mut u32, height: *mut u32, sequence: *mut u64) -> i32 {
    if handle == 0 || width.is_null() || height.is_null() || sequence.is_null() { return -1; }
    let capture = &*(handle as *const CaptureState);
    let Ok(frame) = capture.frame.lock() else { return -1; };
    if frame.closed || capture.control.as_ref().is_none_or(|control| control.is_finished()) { return -1; }
    *width = frame.width; *height = frame.height; *sequence = frame.sequence;
    if frame.bytes.is_empty() { return 0; }
    if output.is_null() || capacity < frame.bytes.len() { return 2; }
    std::ptr::copy_nonoverlapping(frame.bytes.as_ptr(), output, frame.bytes.len());
    1
}

#[no_mangle]
pub unsafe extern "C" fn junshi_wgc_stop(handle: usize) {
    if handle == 0 { return; }
    let mut capture = Box::from_raw(handle as *mut CaptureState);
    if let Some(control) = capture.control.take() { let _ = control.stop(); }
}
