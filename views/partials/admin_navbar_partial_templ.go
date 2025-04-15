// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.857
package partials

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

func AdminNavbar() templ.Component {
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
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div class=\"navbar bg-base-100 shadow-md mb-6 rounded-box\"><div class=\"navbar-start\"><div class=\"dropdown\"><div tabindex=\"0\" role=\"button\" class=\"btn btn-ghost lg:hidden\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-5 w-5\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M4 6h16M4 12h8m-8 6h16\"></path></svg></div><ul tabindex=\"0\" class=\"menu menu-sm dropdown-content mt-3 z-[1] p-2 shadow bg-base-100 rounded-box w-52\"><li><a href=\"/admin\" class=\"font-medium\">Dashboard</a></li><li><a href=\"/admin/users\" class=\"font-medium\">Benutzerverwaltung</a></li><li><a href=\"/admin/images\" class=\"font-medium\">Bilderverwaltung</a></li><li><a href=\"/admin/queues\" class=\"font-medium\">Cache-Monitor</a></li></ul></div><a href=\"/admin\" class=\"btn btn-ghost text-xl\">Admin-Bereich</a></div><div class=\"navbar-center hidden lg:flex\"><ul class=\"menu menu-horizontal px-1\"><li><a href=\"/admin\" class=\"font-medium\">Dashboard</a></li><li><a href=\"/admin/users\" class=\"font-medium\">Benutzerverwaltung</a></li><li><a href=\"/admin/images\" class=\"font-medium\">Bilderverwaltung</a></li><li><a href=\"/admin/queues\" class=\"font-medium\">Cache-Monitor</a></li></ul></div><div class=\"navbar-end\"><form action=\"/admin/search\" method=\"GET\" class=\"flex items-center space-x-2\"><select name=\"type\" class=\"select select-bordered select-sm\"><option value=\"users\">Benutzer</option> <option value=\"images\">Bilder</option></select><div class=\"form-control\"><input type=\"text\" name=\"q\" placeholder=\"Suchen...\" class=\"input input-bordered input-sm w-full max-w-xs\"></div><button type=\"submit\" class=\"btn btn-sm btn-primary\"><svg xmlns=\"http://www.w3.org/2000/svg\" class=\"h-5 w-5\" fill=\"none\" viewBox=\"0 0 24 24\" stroke=\"currentColor\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2\" d=\"M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z\"></path></svg></button></form></div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
