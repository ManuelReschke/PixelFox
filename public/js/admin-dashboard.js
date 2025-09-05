// Admin Dashboard Charts (HTMX-safe, idempotent)
(function(){
  function ensureChartLoader(cb){
    if (window.Chart) { cb(); return; }
    if (typeof window._loadChartJs !== 'function') {
      window._loadChartJs = function(callback){
        if (window.Chart) { callback(); return; }
        var s = document.createElement('script');
        s.src = '/js/chart.umd.min.js';
        s.onload = callback;
        document.head.appendChild(s);
      };
    }
    window._loadChartJs(cb);
  }

  function buildCharts(){
    try {
      if (!window.Chart) { setTimeout(buildCharts, 100); return; }

      var imageEl = document.getElementById('imageStatsData');
      var userEl  = document.getElementById('userStatsData');
      if (!imageEl || !userEl) { setTimeout(buildCharts, 100); return; }

      var imageStats = JSON.parse(imageEl.textContent || '[]');
      var userStats  = JSON.parse(userEl.textContent || '[]');

      var imageData = {
        labels: imageStats.map(function(s){ return s.date; }),
        datasets: [{
          label: 'Bilder',
          data: imageStats.map(function(s){ return s.count; }),
          backgroundColor: 'rgba(34,197,94,.6)',
          borderColor: 'rgba(34,197,94,1)',
          borderWidth: 1
        }]
      };

      var userData = {
        labels: userStats.map(function(s){ return s.date; }),
        datasets: [{
          label: 'Benutzer',
          data: userStats.map(function(s){ return s.count; }),
          backgroundColor: 'rgba(59,130,246,.6)',
          borderColor: 'rgba(59,130,246,1)',
          borderWidth: 1
        }]
      };

      var options = {
        responsive: true,
        maintainAspectRatio: false,
        scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
        plugins: { legend: { display: true } }
      };

      var imgCanvas = document.getElementById('imageChart');
      if (imgCanvas) {
        if (window.imageChart && typeof window.imageChart.destroy === 'function') {
          window.imageChart.destroy();
        }
        window.imageChart = new Chart(imgCanvas.getContext('2d'), { type: 'bar', data: imageData, options: options });
      }

      var userCanvas = document.getElementById('userChart');
      if (userCanvas) {
        if (window.userChart && typeof window.userChart.destroy === 'function') {
          window.userChart.destroy();
        }
        window.userChart = new Chart(userCanvas.getContext('2d'), { type: 'bar', data: userData, options: options });
      }
    } catch (e) {
      console.error('Error building charts:', e);
      setTimeout(buildCharts, 500);
    }
  }

  function initAdminDashboardCharts(){
    // Only proceed on admin dashboard pages that include the canvases
    if (!document.getElementById('imageChart') && !document.getElementById('userChart')) return;
    ensureChartLoader(function(){ buildCharts(); });
  }

  // Expose for debugging if needed
  window._pxf_initAdminDashboardCharts = initAdminDashboardCharts;

  // DOM + HTMX hooks
  document.addEventListener('DOMContentLoaded', function(){ requestAnimationFrame(initAdminDashboardCharts); });
  function reinit(){ requestAnimationFrame(function(){ setTimeout(initAdminDashboardCharts, 30); }); }
  document.addEventListener('htmx:load', reinit);
  document.addEventListener('htmx:afterSwap', reinit);
  document.addEventListener('htmx:afterSettle', reinit);
})();
