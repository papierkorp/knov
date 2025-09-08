# Operation

Browser Request
Internet
Server receives Request and sends Response
Internet
Browser receives Response

**Example:**

URL: `http://user:password@tryhackme.com:80/view-room?id=1#task3`

- URL Details
  - http = scheme / Protocoll
  - user:password = User/Authentication
  - tryhackme.com = Host/Domain/IP
  - 80 = Port
  - view-room = path, filename/location of the ressource
  - ?id = Query String, will be send to path
  - \#task3 = Fragment, Reference on path site
- Request
  - Possible Methods
    - GET = Collect Informations
    - POST = Send Data for new Entrys
    - PUT = Send Data to update Informations
    - DELETE = Delete Data/Informations
  - Procedure for Example
    -   `GET / HTTP/1.1`
    -   `Host: tryhackme.com`
    -   `User-Agent: Mozilla/5.0 Firefox/87.0`
    -   `Referer: https://tryhackme.com/`
- Response
  -   `HTTP/1.1 200 OK`
  -   `Server: nginx/1.15.8`
  -   `Date: Fri, 09 Apr 2021 13:34:03 GMT`
  -   `Content-Type: text/html`
  -  ` Content-Length: 98`
- Header
  - from Client to Server
    - Host = if mor Websites are hosted, else Standard
    - User-Agent = Browser Software
    - Content-Length
    - Accept-Encoding = kind of Encoding
    - Cookie = Data send to Server to remember Informations
  - from Server to Client
    - set-cookie
    - cache-control = how long Data will be saved in Cache
    - content-type = Kind of the Data (HTML, CSS, JS, Images...)
    - content-enconding = Data compression
- Cookies
  - Data is saved on local Computer and will be Send to Server after request
  - In Browser: Developer Tools - Network - Cookie Tab

Header Examples:

```
POST /login HTTP/1.1
Host: tryhackme.com
User-Agent: Mozilla/5.0 Firefox/87.0
Content-Length: 33

username=thm&password=letmein
```

```
GET /room?username=thm&password=letmein HTTP/1.1
Host: tryhackme.com
User-Agent: Mozilla/5.0 Firefox/87.0
Content-Length: 33
```

```
PUT /user/2 HTTP/1.1
Host: tryhackme.com
User-Agent: Mozilla/5.0 Firefox/87.0
Content-Length: 14

username=admin
```

# Javascript

```
<script src="/location/of/javascript_file.js"></script>
```

# HTTP - HyperText Transfer Protocol

Rules for Communication between Webserver and Client (Browser)

Status Codes:

-   100-199 = Informations (part of Request is accepted)
-   200-299 = Success (Request succesfull)
  - 200 = Ok Message
  - 201 = Request Created Message
-   300-399 = Redirection
  - 301 = Permanent Redirect
  - 302 = Temporary Redirect
-   400-499 = Client Errors
  -   400 = Bad Request
  -   401 = Not Authorised
  -   403 = Not allowed
  -   404 = Page not Found
  -   405 = Method not allowed
-   500-599 = Server Errors
  -   500 = Internal Service Error (server doesnt know what to do)
  -   503 = Service unavailable (overloaded/maintenance)

# HTTPS - HyperText Transfer Protocol Secure

same as [HTTP](#HTTP - HyperText Transfer Protocol) just with Data encrypted


# File Filtering

- Client Side Filtering
  - per Browser
  - can easily be avoided
- Server Side Filtering
  - more difficult to evade
  - nearly impossible to completly evade -> use payloads to avoid present filter

- Extension Validation
  - easy to change
  - blacklist/whitelist
- File Type Filtering
  - MIME Validation (Windows)
    - Format: `<type>/<subtype> = Content-Type: image/jpeg`
  - Magic Number Validation (Linux)
    - Magic Numer of File = String of Bytes at the beginning of the File Content
    - e.g. PNG = `89 50 4E 47 0D 0A 1 A 0A`
    - [List of Signatures in Wikipedia](https://en.wikipedia.org/wiki/List_of_file_signatures)
- File Length Filtering
- Filte Name Filtering
  - checking if there is a file with a similiar name
  - filtering for special chars
- File Content Filtering
  - is complex