// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.906
package admin_views

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"strconv"
)

func dashboardContent(totalUsers int, totalImages int, recentUsers []models.User, imageStats []models.DailyStats, userStats []models.DailyStats) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div class=\"mb-8\"><h1 class=\"text-3xl font-bold mb-2\">Admin Dashboard</h1><p class=\"opacity-75\">Verwalte deine PixelFox-Anwendung</p></div><div class=\"grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8\"><!-- Stats Card: Total Users --><div class=\"bg-base-200 rounded-lg shadow p-6\"><div class=\"flex items-center\"><div class=\"p-3 rounded-full bg-blue-100 text-blue-600\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-8 w-8\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z\"></path></svg></div><div class=\"ml-4\"><h2 class=\"text-sm font-medium opacity-75\">Benutzer gesamt</h2><p class=\"text-2xl font-semibold\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var2 string
		templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(strconv.Itoa(totalUsers))
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 25, Col: 65}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "</p></div></div></div><!-- Stats Card: Total Images --><div class=\"bg-base-200 rounded-lg shadow p-6\"><div class=\"flex items-center\"><div class=\"p-3 rounded-full bg-green-100 text-green-600\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-8 w-8\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z\"></path></svg></div><div class=\"ml-4\"><h2 class=\"text-sm font-medium opacity-75\">Bilder gesamt</h2><p class=\"text-2xl font-semibold\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var3 string
		templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(strconv.Itoa(totalImages))
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 40, Col: 66}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "</p></div></div></div><!-- Quick Actions --><div class=\"bg-base-200 rounded-lg shadow p-6\"><h2 class=\"text-sm font-medium opacity-75 mb-4\">Schnellzugriff</h2><div class=\"space-y-2\"><a href=\"/admin/users\" class=\"flex items-center text-blue-600 hover:text-blue-800\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-5 w-5 mr-2\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z\"></path></svg> Benutzerverwaltung</a> <a href=\"/admin/images\" class=\"flex items-center text-green-600 hover:text-green-800\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-5 w-5 mr-2\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z\"></path></svg> Bilderverwaltung</a></div></div></div><!-- Charts --><div class=\"grid grid-cols-1 md:grid-cols-2 gap-6 mb-8\"><!-- Image Chart --><div class=\"bg-base-200 rounded-lg shadow p-6\"><h2 class=\"text-lg font-medium mb-4\">Bilder pro Tag (letzte 7 Tage)</h2><div class=\"h-64\"><canvas id=\"imageChart\"></canvas></div></div><!-- User Chart --><div class=\"bg-base-200 rounded-lg shadow p-6\"><h2 class=\"text-lg font-medium mb-4\">Benutzer pro Tag (letzte 7 Tage)</h2><div class=\"h-64\"><canvas id=\"userChart\"></canvas></div></div></div><!-- Recent Users --><div class=\"bg-base-200 rounded-lg shadow overflow-hidden\"><div class=\"px-6 py-4 border-b border-base-300\"><h2 class=\"text-lg font-medium\">Neueste Benutzer</h2></div><div class=\"divide-y divide-base-300\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if len(recentUsers) == 0 {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "<div class=\"px-6 py-4 opacity-75 text-center\">Keine Benutzer gefunden</div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			for _, user := range recentUsers {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "<div class=\"px-6 py-4\"><div class=\"flex items-center\"><div class=\"flex-shrink-0\"><div class=\"h-10 w-10 rounded-full bg-base-300 flex items-center justify-center opacity-75\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var4 string
				templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(string(user.Name[0]))
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 98, Col: 31}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "</div></div><div class=\"ml-4\"><div class=\"text-sm font-medium\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var5 string
				templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(user.Name)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 102, Col: 52}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "</div><div class=\"text-sm opacity-75\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var6 string
				templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(user.Email)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 103, Col: 52}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "</div></div><div class=\"ml-auto\"><span class=\"px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var7 string
				templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(user.Role)
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/admin_views/dashboard.templ`, Line: 107, Col: 20}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "</span></div></div></div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "</div></div><!-- JSON-Daten für Charts -->")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templ.JSONScript("imageStatsData", imageStats).Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templ.JSONScript("userStatsData", userStats).Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "<!-- Chart.js Loader (OOB) --><script hx-swap-oob=\"true\" id=\"chartjs-loader\">\n\t\tfunction _loadChartJs(callback){\n\t\t\tif(window.Chart){ callback(); return; }\n\t\t\tvar s=document.createElement('script');\n\t\t\ts.src='https://cdn.jsdelivr.net/npm/chart.js';\n\t\t\ts.onload=callback;\n\t\t\tdocument.head.appendChild(s);\n\t\t}\n\t</script><script>\n\t\t// ensure loader helper exists (may run before OOB script)\n\t\tif (typeof _loadChartJs === 'undefined') {\n\t\t\tfunction _loadChartJs(cb){\n\t\t\t\tif(window.Chart){ cb(); return; }\n\t\t\t\tvar s=document.createElement('script');\n\t\t\t\ts.src='https://cdn.jsdelivr.net/npm/chart.js';\n\t\t\t\ts.onload=cb;\n\t\t\t\tdocument.head.appendChild(s);\n\t\t\t}\n\t\t}\n\n\t\t// Globale Variablen für die Charts\n\t\tlet imageChart = null;\n\t\tlet userChart = null;\n\n\t\tfunction buildCharts(){\n\t\t\t// Daten aus den JSON-Skripten laden\n\t\t\tconst imageStats = JSON.parse(document.getElementById('imageStatsData').textContent);\n\t\t\tconst userStats = JSON.parse(document.getElementById('userStatsData').textContent);\n\n\t\t\tconst imageData = {\n\t\t\t\tlabels: imageStats.map(stat => stat.date),\n\t\t\t\tdatasets: [{ label:'Bilder', data:imageStats.map(stat=>stat.count), backgroundColor:'rgba(34,197,94,.2)', borderColor:'rgba(34,197,94,1)', borderWidth:1 }]\n\t\t\t};\n\n\t\t\tconst userData = {\n\t\t\t\tlabels: userStats.map(stat => stat.date),\n\t\t\t\tdatasets: [{ label:'Benutzer', data:userStats.map(stat=>stat.count), backgroundColor:'rgba(59,130,246,.2)', borderColor:'rgba(59,130,246,1)', borderWidth:1 }]\n\t\t\t};\n\n\t\t\tconst chartOptions = { responsive:true, maintainAspectRatio:false, scales:{ y:{ beginAtZero:true, ticks:{precision:0} } } };\n\n\t\t\tconst imageCtx=document.getElementById('imageChart');\n\t\t\tif(imageCtx){ if(imageChart){imageChart.destroy();} imageChart=new Chart(imageCtx.getContext('2d'),{type:'bar', data:imageData, options:chartOptions}); }\n\n\t\t\tconst userCtx=document.getElementById('userChart');\n\t\t\tif(userCtx){ if(userChart){userChart.destroy();} userChart=new Chart(userCtx.getContext('2d'),{type:'bar', data:userData, options:chartOptions}); }\n\t\t}\n\n\t\tfunction initCharts(){ _loadChartJs(buildCharts); }\n\n\t\tdocument.addEventListener('DOMContentLoaded', initCharts);\n\t\tdocument.addEventListener('htmx:afterSettle', function(){ if(document.getElementById('imageChart')){ initCharts(); }});\n\t</script>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func Dashboard(totalUsers int, totalImages int, recentUsers []models.User, imageStats []models.DailyStats, userStats []models.DailyStats) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var8 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var8 == nil {
			templ_7745c5c3_Var8 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = AdminLayout(dashboardContent(totalUsers, totalImages, recentUsers, imageStats, userStats)).Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
