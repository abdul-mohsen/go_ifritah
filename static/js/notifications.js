/* eslint-env browser */
/**
 * Notification badge poller.
 *
 * Behaviour:
 *   - On load, fetches /api/notifications/unread-count and updates the bell badge.
 *   - Re-polls every 60s while the tab is visible; backs off to 5min when hidden.
 *   - Stops polling on 401 (user signed out) and on hard errors.
 *   - Provides window.NotificationBadge for other scripts to nudge the count
 *     after a mark-read / mark-all-read action without waiting for the next tick.
 *
 * Wire-up (templates/layouts/base.html):
 *   #notification-bell    — anchor element with data-unread-count-url
 *   #notification-badge   — span shown/hidden + populated with the count
 *
 * Designed to be a small, dependency-free island. Material-style "9+" cap.
 */
(function () {
    "use strict";

    var BELL_ID = "notification-bell";
    var BADGE_ID = "notification-badge";
    var INTERVAL_VISIBLE_MS = 60 * 1000;
    var INTERVAL_HIDDEN_MS = 5 * 60 * 1000;
    var MAX_DISPLAY = 9;

    var bell = document.getElementById(BELL_ID);
    if (!bell) return; // Bell not in this layout — nothing to do.
    var badge = document.getElementById(BADGE_ID);
    if (!badge) return;

    var endpoint = bell.getAttribute("data-unread-count-url") ||
        "/api/notifications/unread-count";

    var stopped = false;
    var timerId = null;
    var inFlight = null;

    function setCount(n) {
        if (typeof n !== "number" || isNaN(n) || n < 0) n = 0;
        if (n === 0) {
            badge.style.display = "none";
            badge.textContent = "";
            badge.removeAttribute("data-count");
            return;
        }
        badge.style.display = "flex";
        badge.setAttribute("data-count", String(n));
        badge.textContent = n > MAX_DISPLAY ? MAX_DISPLAY + "+" : String(n);
    }

    function fetchCount() {
        if (stopped) return Promise.resolve();
        if (inFlight) return inFlight;

        inFlight = fetch(endpoint, {
            credentials: "same-origin",
            headers: { "Accept": "application/json" },
            cache: "no-store"
        })
            .then(function (resp) {
                if (resp.status === 401) {
                    stopped = true;
                    return null;
                }
                if (!resp.ok) return null;
                return resp.json();
            })
            .then(function (data) {
                if (data && typeof data.count === "number") setCount(data.count);
            })
            .catch(function () {
                // Network blip — keep last known count, try again next tick.
            })
            .finally(function () {
                inFlight = null;
            });
        return inFlight;
    }

    function schedule() {
        if (stopped) return;
        if (timerId !== null) clearTimeout(timerId);
        var delay = document.hidden ? INTERVAL_HIDDEN_MS : INTERVAL_VISIBLE_MS;
        timerId = setTimeout(tick, delay);
    }

    function tick() {
        fetchCount().then(schedule);
    }

    document.addEventListener("visibilitychange", function () {
        if (!document.hidden) {
            // Tab became visible — refresh immediately and reset the schedule.
            fetchCount().then(schedule);
        } else {
            schedule();
        }
    });

    // Public hooks for other scripts (notifications page, settings etc.)
    window.NotificationBadge = {
        refresh: fetchCount,
        setCount: setCount,
        decrement: function (by) {
            var current = parseInt(badge.getAttribute("data-count") || "0", 10) || 0;
            setCount(Math.max(0, current - (by || 1)));
        },
        clear: function () { setCount(0); }
    };

    // Kick off after layout settles.
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", function () {
            fetchCount().then(schedule);
        });
    } else {
        fetchCount().then(schedule);
    }
})();
