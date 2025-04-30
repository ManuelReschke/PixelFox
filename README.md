# PixelFox

The official PixelFox.cc repository. This is work in progress.
This project is more of a feasibility study and is intended for learning purposes.

PixelFox is an image sharing platform. Its build with the following technologies:

## Tech Stack

* Infrastructure:
  * Docker v27.3
  * GoLang v1.23
  * MySQL v8.4
  * Dragonfly Cache
* Backend:
  * Framework [GoFiber](https://github.com/gofiber/fiber)
  * Template Engine [Templ](https://github.com/a-h/templ)
  * Database Handling [GORM](https://github.com/go-gorm/gorm)
  * Database Migrations [Golang Migrate](https://github.com/golang-migrate/migrate)
* Frontend:
  * HTML, HTMX, Hyperscript, Javascript, CSS
  * TailwindCSS & DaisyUI
  * SweetAlert2

## Already Done

* [X] setup basic dev env with Docker
  * [X] GoLang Container
  * [X] MySQL Container
  * [X] Dragonfly Cache Container
  * [X] PHPMyAdmin Container
* [X] DEV env setup
* [X] PROD env setup
* [X] Run [air](https://github.com/air-verse/air) (for HotReload) & [templ](https://github.com/a-h/templ) generate --watch in one container with supervisord
* [X] Create templates for index, login, register, contact, about, news, jobs, api
* [X] Prepare API page and include Swagger & OpenAPI UI (github.com/go-openapi)
* [X] User Authentication login & logout
* [X] User Registration

## ToDos

* [] Image viewer page with:
  * [] with download button
  * [] with like button
  * [] with comment form and list of comments
  * [] for image owner only: delete button
  * [] for image owner add tags and description
* [] User Profile page
* [] User Settings page
* [] DB Schema & Models
* [] Basic image upload
* [] Store image information to DB (also image meta data)
* [] Share images via link
* [] Use Open Graph Meta-Tags (OG-Tags) for the image view page
* [] Store images to B2 or other S3 services (or we use juicefs)
* [] Admin Area
