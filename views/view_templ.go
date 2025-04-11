// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.857
package views

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
)

func ImageViewer(model viewmodel.Image) templ.Component {
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
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<section class=\"mx-auto w-fit flex flex-col gap-6 text-center\"><div class=\"card w-[32rem] bg-base-100 shadow-xl\"><figure class=\"px-6 pt-3 pb-3 bg-base-200 bg-opacity-30 rounded-t-xl\"><!-- Optimierte Bildformate mit picture-Element --><picture>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if model.HasAVIF {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<source srcset=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var2 string
			templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(model.PreviewAVIFPath)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 14, Col: 43}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "\" type=\"image/avif\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		if model.HasWebP {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "<source srcset=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var3 string
			templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(model.PreviewWebPPath)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 17, Col: 43}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "\" type=\"image/webp\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "<img loading=\"lazy\" src=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var4 string
		templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(model.PreviewPath)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 19, Col: 47}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "\" alt=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var5 string
		templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(model.DisplayName)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 19, Col: 71}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "\" class=\"rounded-xl max-h-48 object-contain cursor-pointer\" id=\"preview-image\"></picture></figure><div class=\"card-body bg-base-100\"><h2 class=\"card-title mx-auto truncate max-w-full\" title=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var6 string
		templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(model.DisplayName)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 23, Col: 79}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var7 string
		templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(model.DisplayName)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 23, Col: 99}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "</h2><div class=\"mt-4 space-y-3\"><!-- ShareLink Box mit weissem Hintergrund und fetter Schrift --><div class=\"form-control bg-base-200 bg-opacity-50 p-2 rounded border border-base-300\"><div class=\"flex items-center gap-2\"><label class=\"label w-24 justify-start p-0\"><span class=\"label-text font-bold\">Link teilen:</span></label><div class=\"join w-full\"><input id=\"share-link\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full font-bold\" value=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var8 string
		templ_7745c5c3_Var8, templ_7745c5c3_Err = templ.JoinStringErrs(model.ShareURL)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 33, Col: 138}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var8))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "\"> <button class=\"btn btn-primary btn-sm join-item copy-btn\" data-clipboard-target=\"#share-link\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184\"></path></svg></button></div></div></div><!-- Trennlinie nach dem ShareLink --><div class=\"divider my-1\"></div><div class=\"form-control\"><div class=\"flex items-center gap-2\"><label class=\"label w-24 justify-start p-0\"><span class=\"label-text\">HTML</span></label><div class=\"join w-full\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if model.HasAVIF {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "<input id=\"html\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var9 string
			templ_7745c5c3_Var9, templ_7745c5c3_Err = templ.JoinStringErrs("<img src=\"" + model.Domain + model.PreviewAVIFPath + "\" alt=\"" + model.DisplayName + "\" />")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 53, Col: 205}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var9))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else if model.HasWebP {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "<input id=\"html\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var10 string
			templ_7745c5c3_Var10, templ_7745c5c3_Err = templ.JoinStringErrs("<img src=\"" + model.Domain + model.PreviewWebPPath + "\" alt=\"" + model.DisplayName + "\" />")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 55, Col: 205}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var10))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "<input id=\"html\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var11 string
			templ_7745c5c3_Var11, templ_7745c5c3_Err = templ.JoinStringErrs("<img src=\"" + model.Domain + model.PreviewPath + "\" alt=\"" + model.DisplayName + "\" />")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 57, Col: 201}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var11))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 17, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 18, "<button class=\"btn btn-primary btn-sm join-item copy-btn\" data-clipboard-target=\"#html\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184\"></path></svg></button></div></div></div><div class=\"form-control\"><div class=\"flex items-center gap-2\"><label class=\"label w-24 justify-start p-0\"><span class=\"label-text\">BBCode</span></label><div class=\"join w-full\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if model.HasAVIF {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 19, "<input id=\"bbcode\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var12 string
			templ_7745c5c3_Var12, templ_7745c5c3_Err = templ.JoinStringErrs("[img]" + model.Domain + model.PreviewAVIFPath + "[/img]")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 75, Col: 168}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var12))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 20, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else if model.HasWebP {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 21, "<input id=\"bbcode\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var13 string
			templ_7745c5c3_Var13, templ_7745c5c3_Err = templ.JoinStringErrs("[img]" + model.Domain + model.PreviewWebPPath + "[/img]")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 77, Col: 168}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var13))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 22, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 23, "<input id=\"bbcode\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var14 string
			templ_7745c5c3_Var14, templ_7745c5c3_Err = templ.JoinStringErrs("[img]" + model.Domain + model.PreviewPath + "[/img]")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 79, Col: 164}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var14))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 24, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 25, "<button class=\"btn btn-primary btn-sm join-item copy-btn\" data-clipboard-target=\"#bbcode\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184\"></path></svg></button></div></div></div><div class=\"form-control\"><div class=\"flex items-center gap-2\"><label class=\"label w-24 justify-start p-0\"><span class=\"label-text\">Markdown</span></label><div class=\"join w-full\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if model.HasAVIF {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 26, "<input id=\"markdown\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var15 string
			templ_7745c5c3_Var15, templ_7745c5c3_Err = templ.JoinStringErrs("![" + model.DisplayName + "](" + model.Domain + model.PreviewAVIFPath + ")")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 97, Col: 189}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var15))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 27, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else if model.HasWebP {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 28, "<input id=\"markdown\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var16 string
			templ_7745c5c3_Var16, templ_7745c5c3_Err = templ.JoinStringErrs("![" + model.DisplayName + "](" + model.Domain + model.PreviewWebPPath + ")")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 99, Col: 189}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var16))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 29, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 30, "<input id=\"markdown\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var17 string
			templ_7745c5c3_Var17, templ_7745c5c3_Err = templ.JoinStringErrs("![" + model.DisplayName + "](" + model.Domain + model.PreviewPath + ")")
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 101, Col: 185}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var17))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 31, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 32, "<button class=\"btn btn-primary btn-sm join-item copy-btn\" data-clipboard-target=\"#markdown\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184\"></path></svg></button></div></div></div><div class=\"form-control\"><div class=\"flex items-center gap-2\"><label class=\"label w-24 justify-start p-0\"><span class=\"label-text\">Direktlink</span></label><div class=\"join w-full\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if model.HasAVIF {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 33, "<input id=\"direktlink\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var18 string
			templ_7745c5c3_Var18, templ_7745c5c3_Err = templ.JoinStringErrs(model.Domain + model.PreviewAVIFPath)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 119, Col: 151}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var18))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 34, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else if model.HasWebP {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 35, "<input id=\"direktlink\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var19 string
			templ_7745c5c3_Var19, templ_7745c5c3_Err = templ.JoinStringErrs(model.Domain + model.PreviewWebPPath)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 121, Col: 151}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var19))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 36, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 37, "<input id=\"direktlink\" type=\"text\" readonly class=\"input input-bordered input-sm join-item w-full\" value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var20 string
			templ_7745c5c3_Var20, templ_7745c5c3_Err = templ.JoinStringErrs(model.Domain + model.PreviewPath)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 123, Col: 147}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var20))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 38, "\"> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 39, "<button class=\"btn btn-primary btn-sm join-item copy-btn\" data-clipboard-target=\"#direktlink\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184\"></path></svg></button></div></div></div></div><div class=\"card-actions justify-center mt-4\"><a href=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var21 templ.SafeURL = templ.URL(model.Domain + model.OriginalPath)
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var21)))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 40, "\" download=\"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var22 string
		templ_7745c5c3_Var22, templ_7745c5c3_Err = templ.JoinStringErrs(model.DisplayName)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 136, Col: 87}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var22))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 41, "\" target=\"_blank\" rel=\"noopener noreferrer\" class=\"btn btn-primary btn-sm w-full\"><svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4 mr-2\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3\"></path></svg> Original herunterladen</a> <a href=\"/\" class=\"btn btn-primary btn-sm\">Neues Bild hochladen</a></div></div></div></section><!-- Modal für Bildanzeige mit lazy loading --><dialog id=\"image-modal\" class=\"modal\"><div class=\"modal-box max-w-5xl\"><form method=\"dialog\"><button class=\"btn btn-sm btn-circle btn-ghost absolute right-2 top-2\">×</button></form><div class=\"py-4 flex justify-center\"><!-- Loading spinner that shows initially --><div id=\"loading-spinner\" class=\"flex flex-col items-center justify-center\"><span class=\"loading loading-spinner loading-lg text-primary\"></span><p class=\"mt-2\">Loading optimized image...</p></div><!-- Picture element that will be populated via JavaScript --><picture id=\"modal-picture\" class=\"hidden\"><!-- Sources will be added dynamically --><img id=\"modal-image\" class=\"max-h-[80vh] object-contain\" alt=\"\"></picture></div></div></dialog><script src=\"https://cdnjs.cloudflare.com/ajax/libs/clipboard.js/2.0.11/clipboard.min.js\"></script><script>\n\t\t// Check if imagePaths is already defined to avoid redeclaration\n\t\tif (typeof window.pixelFoxImagePaths === 'undefined') {\n\t\t\t// Create a global variable to store image paths\n\t\t\twindow.pixelFoxImagePaths = {};\n\t\t}\n\n\t\t// Update image paths for current image\n\t\twindow.pixelFoxImagePaths = {\n\t\t\tavif: \"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var23, templ_7745c5c3_Err := templruntime.ScriptContentInsideStringLiteral(model.OptimizedAVIFPath)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 180, Col: 36}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var23)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 42, "\",\n\t\t\twebp: \"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var24, templ_7745c5c3_Err := templruntime.ScriptContentInsideStringLiteral(model.OptimizedWebPPath)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 181, Col: 36}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var24)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 43, "\",\n\t\t\toriginal: \"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var25, templ_7745c5c3_Err := templruntime.ScriptContentInsideStringLiteral(model.OriginalPath)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 182, Col: 35}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var25)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 44, "\",\n\t\t\thasAvif: ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var26, templ_7745c5c3_Err := templruntime.ScriptContentOutsideStringLiteral(model.HasAVIF)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 183, Col: 28}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var26)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 45, ",\n\t\t\thasWebp: ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var27, templ_7745c5c3_Err := templruntime.ScriptContentOutsideStringLiteral(model.HasWebP)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 184, Col: 28}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var27)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 46, ",\n\t\t\tdisplayName: \"")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Var28, templ_7745c5c3_Err := templruntime.ScriptContentInsideStringLiteral(model.DisplayName)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `views/view.templ`, Line: 185, Col: 37}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ_7745c5c3_Var28)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 47, "\"\n\t\t};\n\n\t\t// Function to open the modal and load images on demand\n\t\tfunction openImageModal() {\n\t\t\t// Show the modal first\n\t\t\tconst modal = document.getElementById('image-modal');\n\t\t\tmodal.showModal();\n\t\t\t\n\t\t\t// Show loading spinner, hide picture\n\t\t\tconst spinner = document.getElementById('loading-spinner');\n\t\t\tconst picture = document.getElementById('modal-picture');\n\t\t\tspinner.classList.remove('hidden');\n\t\t\tpicture.classList.add('hidden');\n\t\t\t\n\t\t\t// Get the image element\n\t\t\tconst img = document.getElementById('modal-image');\n\t\t\t\n\t\t\t// Clear existing sources\n\t\t\twhile (picture.firstChild) {\n\t\t\t\tif (picture.firstChild !== img) {\n\t\t\t\t\tpicture.removeChild(picture.firstChild);\n\t\t\t\t} else {\n\t\t\t\t\tbreak;\n\t\t\t\t}\n\t\t\t}\n\t\t\t\n\t\t\t// Create image object to preload\n\t\t\tconst preloadImg = new Image();\n\t\t\t\n\t\t\t// Determine which optimized format to use\n\t\t\tlet imageSrc = \"\";\n\t\t\tif (window.pixelFoxImagePaths.hasAvif) {\n\t\t\t\t// Use AVIF if available\n\t\t\t\timageSrc = window.pixelFoxImagePaths.avif;\n\t\t\t\tconst avifSource = document.createElement('source');\n\t\t\t\tavifSource.srcset = window.pixelFoxImagePaths.avif;\n\t\t\t\tavifSource.type = 'image/avif';\n\t\t\t\tpicture.insertBefore(avifSource, img);\n\t\t\t} else if (window.pixelFoxImagePaths.hasWebp) {\n\t\t\t\t// Use WebP if AVIF not available\n\t\t\t\timageSrc = window.pixelFoxImagePaths.webp;\n\t\t\t\tconst webpSource = document.createElement('source');\n\t\t\t\twebpSource.srcset = window.pixelFoxImagePaths.webp;\n\t\t\t\twebpSource.type = 'image/webp';\n\t\t\t\tpicture.insertBefore(webpSource, img);\n\t\t\t} else {\n\t\t\t\t// Fallback to original only if no optimized version exists\n\t\t\t\timageSrc = window.pixelFoxImagePaths.original;\n\t\t\t}\n\t\t\t\n\t\t\t// When image loads, hide spinner and show image\n\t\t\tpreloadImg.onload = function() {\n\t\t\t\tspinner.classList.add('hidden');\n\t\t\t\tpicture.classList.remove('hidden');\n\t\t\t};\n\t\t\t\n\t\t\t// Set image src and alt\n\t\t\timg.alt = window.pixelFoxImagePaths.displayName;\n\t\t\timg.src = imageSrc; // Use the selected optimized format\n\t\t\tpreloadImg.src = imageSrc; // Start loading\n\t\t}\n\n\t\t// Function to initialize event listeners\n\t\tfunction initializeEventListeners() {\n\t\t\t// Add click event to preview image\n\t\t\tconst previewImage = document.getElementById('preview-image');\n\t\t\tif (previewImage) {\n\t\t\t\t// Remove any existing listeners to avoid duplicates\n\t\t\t\tpreviewImage.removeEventListener('click', openImageModal);\n\t\t\t\t// Add the click event listener\n\t\t\t\tpreviewImage.addEventListener('click', openImageModal);\n\t\t\t}\n\t\t}\n\n\t\t// Initialize clipboard and event listeners when DOM is loaded\n\t\tdocument.addEventListener('DOMContentLoaded', function() {\n\t\t\tinitializeEventListeners();\n\n\t\t\t// Initialize clipboard\n\t\t\tvar clipboard = new ClipboardJS('.copy-btn');\n\t\t\t\n\t\t\tclipboard.on('success', function(e) {\n\t\t\t\tconst button = e.trigger;\n\t\t\t\tconst originalHTML = button.innerHTML;\n\t\t\t\t\n\t\t\t\tbutton.innerHTML = `\n\t\t\t\t\t<svg xmlns=\"http://www.w3.org/2000/svg\" fill=\"none\" viewBox=\"0 0 24 24\" stroke-width=\"1.5\" stroke=\"currentColor\" class=\"w-4 h-4\">\n\t\t\t\t\t\t<path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M4.5 12.75l6 6 9-13.5\" />\n\t\t\t\t\t</svg>\n\t\t\t\t`;\n\t\t\t\t\n\t\t\t\tsetTimeout(function() {\n\t\t\t\t\tbutton.innerHTML = originalHTML;\n\t\t\t\t}, 1500);\n\t\t\t\t\n\t\t\t\te.clearSelection();\n\t\t\t});\n\t\t\t\n\t\t\tclipboard.on('error', function(e) {\n\t\t\t\tconsole.error('Fehler beim Kopieren: ', e.action);\n\t\t\t});\n\t\t});\n\n\t\t// Handle HTMX after-swap event to reinitialize event listeners\n\t\tdocument.body.addEventListener('htmx:afterSwap', function(event) {\n\t\t\tinitializeEventListeners();\n\t\t});\n\t</script>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
