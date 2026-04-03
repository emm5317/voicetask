// VoiceTask — client-side JavaScript
// Alpine.js components, HTMX handlers, SortableJS init

// Theme toggle (used by layout body x-data)
function themeToggle() {
    return {
        theme: localStorage.getItem('vt-theme') || 'dark',
        toggle() {
            this.theme = this.theme === 'dark' ? 'light' : 'dark';
            localStorage.setItem('vt-theme', this.theme);
        }
    };
}

// Clock display
function clock() {
    return {
        time: '', date: '',
        init() { this.tick(); setInterval(() => this.tick(), 60000); },
        tick() {
            var n = new Date();
            this.time = n.toLocaleTimeString('en-US', {hour:'numeric',minute:'2-digit'});
            this.date = n.toLocaleDateString('en-US', {weekday:'long',month:'short',day:'numeric'});
        }
    };
}

// Global flag for listening state (avoids Alpine store issues)
window._vtListening = false;

// Voice capture (task input via speech recognition)
function voiceCapture() {
    return {
        listening: false,
        transcript: '',
        submitting: false,
        recognition: null,
        speechSupported: false,
        silenceTimer: null,

        init() {
            var saved = localStorage.getItem('vt-draft');
            if (saved) this.transcript = saved;
            this.$watch('transcript', function(v) { localStorage.setItem('vt-draft', v); });

            var SR = window.SpeechRecognition || window.webkitSpeechRecognition;
            if (!SR) return;
            this.speechSupported = true;

            this.recognition = new SR();
            this.recognition.continuous = true;
            this.recognition.interimResults = true;
            this.recognition.lang = 'en-US';

            var self = this;
            this.recognition.onresult = function(e) {
                self.transcript = Array.from(e.results).map(function(r){return r[0].transcript;}).join('');
                self.resetSilenceTimer();
            };
            this.recognition.onend = function() {
                self.clearSilenceTimer();
                if (self.listening) {
                    self.setListening(false);
                    if (self.transcript.trim()) self.submitTask();
                }
            };
            this.recognition.onerror = function(e) {
                if (e.error !== 'no-speech') {
                    self.clearSilenceTimer();
                    self.setListening(false);
                }
            };
        },

        resetSilenceTimer() {
            this.clearSilenceTimer();
            var self = this;
            this.silenceTimer = setTimeout(function() {
                if (self.listening && self.transcript.trim()) {
                    self.recognition.stop();
                }
            }, 3000);
        },

        clearSilenceTimer() {
            if (this.silenceTimer) { clearTimeout(this.silenceTimer); this.silenceTimer = null; }
        },

        setListening(val) {
            this.listening = val;
            window._vtListening = val;
            var el = document.getElementById('listening-indicator');
            if (el) el.style.display = val ? 'flex' : 'none';
        },

        toggleListening() {
            if (this.listening) {
                this.clearSilenceTimer();
                this.recognition.stop();
            } else {
                this.transcript = '';
                this.setListening(true);
                this.recognition.start();
            }
        },

        submitTask() {
            var text = this.transcript.trim();
            if (!text || this.submitting) return;
            this.submitting = true;
            var self = this;
            self.transcript = '';
            localStorage.removeItem('vt-draft');

            htmx.ajax('POST', '/tasks', {
                target: '#task-list', swap: 'outerHTML',
                values: { input: text, source: self.listening ? 'voice' : 'text' }
            }).then(function() { self.submitting = false; })
              .catch(function() { self.submitting = false; });
        }
    };
}

// Date navigation helpers
function prevDay(dateStr) {
    var d = new Date(dateStr + 'T12:00:00');
    d.setDate(d.getDate() - 1);
    return d.toISOString().split('T')[0];
}
function nextDay(dateStr) {
    var d = new Date(dateStr + 'T12:00:00');
    d.setDate(d.getDate() + 1);
    return d.toISOString().split('T')[0];
}

// Timer display — live-ticking HH:MM:SS
// Can receive args directly or read from data-* attributes on $el
function timerDisplay(startTimeISO, matterLabel) {
    return {
        startTime: null,
        matterLabel: '',
        elapsed: '00:00:00',
        ticker: null,

        init() {
            // Support both direct args and data-* attributes
            var iso = startTimeISO || (this.$el && this.$el.dataset.startTime) || '';
            this.matterLabel = matterLabel || (this.$el && this.$el.dataset.matterLabel) || '';
            this.startTime = iso ? new Date(iso) : null;
            if (this.startTime) this.startTicking();
        },

        startTicking() {
            var self = this;
            self.tick();
            self.ticker = setInterval(function() { self.tick(); }, 1000);
        },

        tick() {
            if (!this.startTime) { this.elapsed = '00:00:00'; return; }
            var diff = Math.floor((Date.now() - this.startTime.getTime()) / 1000);
            if (diff < 0) diff = 0;
            var h = Math.floor(diff / 3600);
            var m = Math.floor((diff % 3600) / 60);
            var s = diff % 60;
            this.elapsed = String(h).padStart(2,'0') + ':' + String(m).padStart(2,'0') + ':' + String(s).padStart(2,'0');
            if (this.matterLabel) {
                document.title = this.elapsed + ' \u2014 ' + this.matterLabel + ' | VoiceTask';
            }
        },

        destroy() {
            if (this.ticker) { clearInterval(this.ticker); this.ticker = null; }
            document.title = 'VoiceTask';
        }
    };
}

// Voice note for time entries
// Can receive entryId directly or read from data-entry-id on $el
function timeVoiceNote(entryId) {
    return {
        entryId: '',
        listening: false,
        transcript: '',
        recognition: null,
        speechSupported: false,
        silenceTimer: null,

        init() {
            this.entryId = entryId || (this.$el && this.$el.dataset.entryId) || '';

            var SR = window.SpeechRecognition || window.webkitSpeechRecognition;
            if (!SR) return;
            this.speechSupported = true;

            this.recognition = new SR();
            this.recognition.continuous = true;
            this.recognition.interimResults = true;
            this.recognition.lang = 'en-US';

            var self = this;
            this.recognition.onresult = function(e) {
                self.transcript = Array.from(e.results).map(function(r){ return r[0].transcript; }).join('');
                self.resetSilenceTimer();
            };
            this.recognition.onend = function() {
                self.clearSilenceTimer();
                if (self.listening) {
                    self.listening = false;
                    if (self.transcript.trim()) self.submitNote();
                }
            };
            this.recognition.onerror = function(e) {
                if (e.error !== 'no-speech') {
                    self.clearSilenceTimer();
                    self.listening = false;
                }
            };
        },

        resetSilenceTimer() {
            this.clearSilenceTimer();
            var self = this;
            this.silenceTimer = setTimeout(function() {
                if (self.listening && self.transcript.trim()) {
                    self.recognition.stop();
                }
            }, 3000);
        },

        clearSilenceTimer() {
            if (this.silenceTimer) { clearTimeout(this.silenceTimer); this.silenceTimer = null; }
        },

        toggleListening() {
            if (this.listening) {
                this.clearSilenceTimer();
                this.recognition.stop();
            } else {
                this.transcript = '';
                this.listening = true;
                this.recognition.start();
            }
        },

        submitNote() {
            var text = this.transcript.trim();
            if (!text || !this.entryId) return;
            this.transcript = '';
            htmx.ajax('PATCH', '/time/' + this.entryId, {
                target: '#time-panel', swap: 'outerHTML',
                values: { action: 'description', description: text, raw_transcript: text }
            });
        }
    };
}

// CSRF token helper — reads csrf_ cookie value
function getCsrfToken() {
    var match = document.cookie.match(/(^|; )csrf_=([^;]*)/);
    return match ? decodeURIComponent(match[2]) : '';
}

// Inject CSRF token into all HTMX requests
document.addEventListener('htmx:configRequest', function(e) {
    e.detail.headers['X-CSRF-Token'] = getCsrfToken();
});

// HTMX error toast — must wait for body to exist since this script loads in <head>
document.addEventListener('DOMContentLoaded', function() {
    document.body.addEventListener('htmx:responseError', function() {
        var t = document.getElementById('error-toast');
        if (t) {
            t.style.display='block'; t.style.animation='fadeIn 0.2s ease both';
            setTimeout(function(){ t.style.display='none'; }, 3000);
        }
    });
});

// SortableJS init
function initSortable() {
    document.querySelectorAll('.sortable-group').forEach(function(el) {
        if (el._sortable) el._sortable.destroy();
        el._sortable = new Sortable(el, {
            animation: 200, ghostClass: 'sortable-ghost',
            onEnd: function() {
                var items = [];
                el.querySelectorAll('.task-row[data-id]').forEach(function(row, i) {
                    items.push({id: row.dataset.id, sort_order: i});
                });
                fetch('/tasks/reorder', {
                    method: 'POST',
                    headers: {'Content-Type':'application/json', 'X-CSRF-Token': getCsrfToken()},
                    body: JSON.stringify(items)
                });
            }
        });
    });
}
document.addEventListener('DOMContentLoaded', initSortable);
document.addEventListener('htmx:afterSwap', function(e) {
    if (e.detail.target && e.detail.target.id === 'task-list') initSortable();
});
