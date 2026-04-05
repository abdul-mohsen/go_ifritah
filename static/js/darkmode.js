// ── Dark Mode Toggle ──
(function() {
    function getTheme() {
        return localStorage.getItem('afrita_theme') || 'light';
    }

    function applyTheme(theme) {
        var html = document.documentElement;
        if (theme === 'dark') {
            html.setAttribute('data-theme', 'dark');
        } else {
            html.removeAttribute('data-theme');
        }
        updateIcon(theme);
        localStorage.setItem('afrita_theme', theme);
    }

    function updateIcon(theme) {
        var icon = document.getElementById('dark-icon');
        if (icon) {
            icon.textContent = theme === 'dark' ? '☀️' : '🌙';
        }
    }

    var _existingToggle = window.toggleDarkMode;
    window.toggleDarkMode = function() {
        var current = getTheme();
        var next = current === 'dark' ? 'light' : 'dark';
        applyTheme(next);
        // Call any previously registered toggle handler (e.g., chart updates)
        if (_existingToggle) _existingToggle();
        // Call chart theme update if available (registered by dashboard)
        if (window._updateChartTheme) {
            setTimeout(window._updateChartTheme, 100);
        }
    };

    // Apply immediately (before DOMContentLoaded to prevent flash)
    var saved = getTheme();
    if (saved === 'dark') {
        document.documentElement.setAttribute('data-theme', 'dark');
    }

    // Update icon after DOM is ready
    document.addEventListener('DOMContentLoaded', function() {
        updateIcon(getTheme());
    });
})();
