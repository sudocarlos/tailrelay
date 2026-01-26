// Minimal JavaScript for Tailrelay Web UI
// Auto-refresh functionality

document.addEventListener('DOMContentLoaded', function() {
    // Auto-refresh status every 5 seconds if on dashboard
    if (window.location.pathname === '/') {
        setInterval(function() {
            location.reload();
        }, 5000);
    }
});
