# SSH

=> Application Protocol to Connect to a Remote Maschine 

```
ssh -i id_rsa username@10.10.149.209
```


# SMB

= Server Message Protocol = Client-Server, response-request Protokoll for data exchange

Establish a connection:

```
smbclient //10.10.10.2/secret -u suit -p 445
```

- /secret = share name
- -u = User
- -p = Port

# Telnet

=> Application Protocol to Connect to a Remote Maschine without Security, everything in textual form

Replaced with SSH

Usage:

```
telnet [ip] [port]
```

 CVE = Common Vulnerabilities and Exposures für Telnet
- [https://www.cvedetails.com/](https://www.cvedetails.com/ "https://www.cvedetails.com/")
- [https://cve.mitre.org/](https://cve.mitre.org/ "https://cve.mitre.org/")

# FTP

=> File Transfer Protokol (client-server model)

- Command Channel + Data Channel
- Client initiates Connection, Server validates login credentials and opens session
  - active Connection = Client opens port and listens, server must build active Connection
  - passive Connection = Server opens port and listens, client must build active Connection
  - able to send data and commands at the same time

Usage:

```
ftp [ip]
```

- login: anonymous
- pw: (leer)

# NFS

=> Network File System, mounten von entfernten Ordnern

- Client requests folder from Remote Host to mount onto local host - nfsd (NFS deamon) connects per RPC
- Server checks if User is allowd to mount and returns a file which identifies every Folder/File
- If a File is used per NFS, a RPC Call is send to the nfsd with the following Parameters: File Handle, Name of File, user ID, users Group ID

Usage:

```
mkdir /tmp/mount
sudo mount -t nfs 10.10.128.227:home /tmp/mount/ -nolock
```

- mount the NFS /home to /tmp/mount

## root squashing

nfs doesnt allow root access except for the root Folder.
root User will become a "nfsnobody" User

# SMTP

=> Simple Mail Transfer Protocol

- verify email sender, sends emails, send emails to Sender if email couldnt be send
- POP = email loads from server and will be deleted
- IMAP = emails will be synchronized

Procedure:

User Mail -> SMTP Handshake -> SMTP Server checks if Sender Domain + Recipient Domain are equal -> SMTP Server builds a Connection per SMTP to Recipient -> checks for SMTP Queue -> verifies email + send -> POP/IMAP Server receives email -> Recipient gets email

# MySQL

=> is a RDBMS - Relational Database Management System

Consists of Server + Utility Programms

Procedure:

MySQL creates Database -> Client requests with SQL Statements -> Server answers with Informations/Data

Usage:

```
mysql -h [IP] -u [username] -p
```

- schema = synonym to database
- hashes = crypto Algorithmus to change an Input with variable Length to Output with fixed Length
  - index data into a hash table (every hash has its own unique id and points to original data, index is smaller = faster search)