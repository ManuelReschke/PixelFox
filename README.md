# PixelFox

The official PixelFox.cc repository. This is work in progress.
This project is more of a feasibility study and is intended for learning purposes.

PixelFox is an image sharing platform. Its build with the following technologies:

* Backend:
  * GoLang
    * [GoFiber](https://github.com/gofiber/fiber)
    * [Templ](https://github.com/a-h/templ)
    * [GORM](https://github.com/go-gorm/gorm)
  * MySQL
  * Dragonfly Cache
* Frontend:
  * HTML, HTMX, Hyperscript, Javascript, CSS
  * TailwindCSS & DaisyUI
  * SweetAlert2
* Docker

## ToDos

* [X] setup basic dev env with Docker
    * [X] GoLang Container
    * [X] MySQL Container
    * [X] Dragonfly Cache Container
    * [X] PHPMyAdmin Container
* [X] DEV env setup 
* [X] PROD env setup
* [X] Run [air](https://github.com/air-verse/air) (for HotReload) & [templ](https://github.com/a-h/templ) generate --watch in one container with supervisord
* [] DB Schema & Models
* [] User Authentication Login & Logout
* [] User Registration
* [] Basic Image Upload
* [] Share Images via Link
* [] Admin Area
