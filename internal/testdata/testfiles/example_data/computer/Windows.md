# Versions

- End-User
    - Windows XP
    - Windows Vista
    - Windows 7
    - Windows 8.x
    - Windows 10
        - Home
        - Pro
    - Windows 11
- Server
    - Windows Server 2019

# File System

[File Systems Overview](https://learn.microsoft.com/en-us/troubleshoot/windows-client/backup-and-storage/fat-hpfs-and-ntfs-file-systems)

- FAT16/FAT32 (File Allocation Table)
- HPFS (High Performance File System)
- NTFS (New Technology File System) - Currently used
    - Details
        - NTFS is known as a journaling file system. In case of a failure, the file system can automatically repair the folders/files on disk using information stored in a log file. This function is not possible with FAT.
        - Supports files larger than 4GB
        - Set specific permissions on folders and files
        - Folder and file compression
        - Encryption ([Encryption File System](https://docs.microsoft.com/en-us/windows/win32/fileio/file-encryption) or EFS)
        - ADS (Alternate Data Streams)
            - every file has at least one data stream ($DATA) and ADS allows more then one data stream (allows maleware writes to hide data) - [More Information](https://www.malwarebytes.com/blog/news/2015/07/introduction-to-alternate-data-streams)
    - Permissions for files/folders (Rightlick - Properties - Security - Group or user names)

| Permission           | Folders                                                                                                           | Files                                                                                |
|----------------------|-------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------|
| Read                 | Permits viewing and listing of files and subfolders                                                               | Permits viewing or accessing of the files contents                                   |
| Write                | Permits adding of files and subfolders                                                                            | Permits writing to a file                                                            |
| Read & Execute       | Permits viewing and listing of files and subfolders as well as executing of files; inherited by files and folders | Permits viewing and accessing of the files contents as well as executing of the file |
| List Folder Contents | Permits viewing and listing of files and subfolders as well as executing of files; inherited by folders only      | N/A                                                                                  |
| Modify               | Permits reading and writing of files and subfolders; allows deletion of the folder                                | Permits reading and writing of the file; allows deletion of the file                 |
| Full Control         | Permits reading, writing, changing, and deleting of files and subfolders                                          | Permits reading, writing, changing and deleting of the file                          |


# Accounts/Profiles

**See all users**
- Windows Search: `Other users`
- "Win + R" / Rightlick on Start Menu and Run: `lusrmgr.msc`


**Two types of users**
- Administrator
    -  can make changes to the system: add users, delete users, modify groups, modify settings on the system, etc.
- Standard User
    - can only make changes to folders/files attributed to the user & can't perform system-level changes, such as install programs.

**Default Folders for users**
- Desktop
- Documents
- Downloads
- Music
- Pictures

**UAC (User Account Control)**
The default doesnt apply for built-in-local administrator account.
When a user with an account type of administrator logs into a system, the current session doesn't run with elevated permissions. When an operation requiring higher-level privileges needs to execute, the user will be prompted to confirm if they permit the operation to run.

The UAC settings can be changed or even turned off entirely (not recommended).




# Important Folders/Tools

The `%windir%` Environment Variable contains the path to the Windows Operating System.

- `%windir%/system32` - holds important Files, critical for the operating system
- `msconfig` - is for advanced Troubleshooting and helps to diagnose starup issues
    - general = devices and service for Windows to load upon boot
    - boot = boot options
    - services = all configured services
    - startup = empty
    - tools = utilites to configure the os further
- `computermanagement`
    - system tools
        - task scheduler = create and manage common tasks
        - event viewer = view events that have occurred on the computer
            - event types
                - Error = e.g. data loss, loss of functionality
                - Warning = e.g. disk space low
                - Information = e.g. network driver loaded successfully
                - Success Audit = e.g. user successfully logged in
                - Failure Audit = e.g. user access to a network drive failed
            - Default Logs
                - Application = e.g. database application
                - Security = e.g. valid/invalid login attempts
                - System = e.g. failure of driver
                - Custom Log
        - shared folder = see a complete list of shares and folder that others can connect to
            - sessions = list of users connected
            - open files = all currently accessed files
        - local users and groups = local users
        - performance = performance data
        - device manager = view and configure hardware
    - storage
        - windows server backup
        - disk management
            - set up a new drive
            - extend a partition
            - shrink a partition
            - assign or change drive letters
    - services and applications = view properties of service, enable/disable a service
- `msinfo32` (System Information) = displays a comprehensive view of your hardware, system components, and software environment
    - hardware resources
    - components
        - network
            - adapater = shows the ip address
    - software environment
        - environment variables = see all env Variables
            - `Control Panel > System and Security > System > Advanced system settings > Environment Variables`
            - `Settings > System > About > system info > Advanced system settings > Environment Variables`
- `resmon` (Resource Monitor) = displays per-process and aggregate CPU, memory, disk, and network usage information, details about which processes are using individual file handles and modules
- `regedit` ([Windows Registry](https://learn.microsoft.com/en-us/troubleshoot/windows-server/performance/windows-registry-advanced-users)) = central hierarchical database used to store information necessary to configure the system for one or more users, applications, and hardware devices
    - Profiles for each user
    - Applications installed on the computer and the types of documents that each can create
    - Property sheet settings for folders and application icons
    - What hardware exists on the system
    - The ports that are being used.

## Taskmanager

https://www.howtogeek.com/405806/windows-task-manager-the-complete-guide/

## Command Prompt (cmd)

useful Commands:

```bash
hostname
whoami #show current user
ipconfig /? #show ip informations
cls #clear the commandline
netstat -a -b -e #protocol statistics and current TCP/IP Network connections
net #manage network resources
net user #see all subcommands..
```


# Active Directory

The main idea behind a Windows domain is to centralise the administration of common components of a Windows computer network in a single repository called Active Directory (AD). The server that runs the Active Directory services is known as a Domain Controller (DC).

The core of any Windows Domain is the Active Directory Domain Service (AD DS). This service acts as a catalogue that holds the information of all of the "objects" that exist on your network. Amongst the many objects supported by AD, we have users, groups, machines, printers, shares and many others. Let's look at some of them:

security principals: can be authenticated by the domain and assigned privileges over resources

- Users 
    - object (security principals)
    - can be People
    - can be Services (e.g. IIS or MSSQL)
- Machines
    - object (security principals)
    - can be computers
        -  the machine Account is a local Administrator on the assigned computer and is generally not supposed to be accessed by anyone except the computer itself
        - Machine Account passwords are automatically rotated out and are generally comprised of 120 random characters
    - can be identified by `$` at the end of the name
- Security Groups
    - object (security principals)
    - you can define user groups to assign access rights to files or other resources to entire groups instead of single users
    - well known groups - [Complete List here](https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/understand-security-groups)
        - Domain Admin
            - `Users of this group have administrative privileges over the entire domain. By default, they can administer any computer on the domain, including the DCs.`
        - Server Operator
            - `Users in this group can administer Domain Controllers. They cannot change any administrative group memberships.` 
        - Backup Operator
            - `Users in this group are allowed to access any file, ignoring their permissions. They are used to perform backups of data on computers.`
        - Account Operator
            - `Users in this group can create or modify other accounts in the domain.`
        - Domain User
            - `Includes all existing user accounts in the domain.`
        - Domain Computers
            - `Includes all existing computers in the domain.`
        - Domain Controllers
            - `Includes all existing DCs on the domain.`
- printers
- shares
- ...


If you have two domains that share the same namespace (`thm.local` in our example), those domains can be joined into a **Tree**.

If our `thm.local` domain was split into two subdomains for UK and US branches, you could build a tree with a root domain of `thm.local` and two subdomains called `uk.thm.local` and `us.thm.local`, each with its AD, computers and users.

When both companies merge, you will probably have different domain trees for each company, each managed by its own IT department. The union of several trees with different namespaces into the same network is known as a **forest**.

Having multiple domains organised in trees and forest allows you to have a nice compartmentalised network in terms of management and resources. But at a certain point, a user at THM UK might need to access a shared file in one of MHT ASIA servers. For this to happen, domains arranged in trees and forests are joined together by **trust relationships**.

## Management

To configure users, groups or machines in Active Directory, we need to log in to the Domain Controller and run "Active Directory Users and Computers" from the start menu.

These objects are organised in Organizational Units (OUs) which are container objects that allow you to classify users and machines.

Default Container:

- Builtin
    - `Contains default groups available to any Windows host.`
- Computers
    - `Any machine joining the network will be put here by default. You can move them if needed.`
- Domain Controllers
    - `Default OU that contains the DCs in your network.`
    - `Domain Controllers are the third most common device within an Active Directory domain. Domain Controllers allow you to manage the Active Directory Domain. These devices are often deemed the most sensitive devices within the network as they contain hashed passwords for all user accounts within the environment.`
- Users
    - `Default users and groups that apply to a domain-wide context.`
- Managed Service Accounts
    - `Holds accounts used by services in your Windows domain.`
- Workstations
    - `Workstations are one of the most common devices within an Active Directory domain. Each user in the domain will likely be logging into a workstation. This is the device they will use to do their work or normal browsing activities. These devices should never have a privileged user signed into them.`
- Servers
    - `Servers are the second most common device within an Active Directory domain. Servers are generally used to provide services to users or other servers.`


Difference between Security Groups vs OUs

- OUs 
    - `are handy for applying policies to users and computers, which include specific configurations that pertain to sets of users depending on their particular role in the enterprise. Remember, a user can only be a member of a single OU at a time, as it wouldn't make sense to try to apply two different sets of policies to a single user.`
- Security Groups
    - `are used to grant permissions over resources. For example, you will use groups if you want to allow some users to access a shared folder or network printer. A user can be a part of many groups, which is needed to grant access to multiple resources.`


## Group Policies

Managed through: Group Policy Objects (GPO)
GPO's are simply a collection of settings that can be applied to OUs.

To configure GPOs, you can use the **Group Policy Management** tool, available from the start menu. 

To configure Group Policies, you first create a GPO under **Group Policy Objects** and then link it to the GPO where you want the policies to apply.

GPOs are distributed to the network via a network share called `SYSVOL`, which is stored in the DC. All users in a domain should typically have access to this share over the network to sync their GPOs periodically. The SYSVOL share points by default to the `C:\Windows\SYSVOL\sysvol\` directory on each of the DCs in our network.

Once a change has been made to any GPOs, it might take up to 2 hours for computers to catch up. If you want to force any particular computer to sync its GPOs immediately, you can always run the following command on the desired computer: `PS C:\> gpupdate /force`


## Authentication

When using Windows domains, all credentials are stored in the Domain Controllers. Whenever a user tries to authenticate to a service using domain credentials, the service will need to ask the Domain Controller to verify if they are correct. Two protocols can be used for network authentication in windows domains:

- Kerberos 
    - `Used by any recent version of Windows. This is the default protocol in any recent domain.`
    1) User tries logging in via Kerberos
        - The user sends their username and a timestamp encrypted using a key derived from their password to the Key Distribution Center (KDC) service
        - The KDC will create and send back a Ticket Granting Ticket (TGT encrypted with the krbtgt account's password hash and the session key) as well as a Session Key
        - This ticket allows user to request more tickets to access specific services
     2) User wants to connect to a specific service
         - User sends their username and a timestamp encrypted using the Session Key, along with the TGT and a Service Principal Name (SPN) to ask the KDC for a Ticket Granting Service (TGS are tickets that allow connection only to the specific service they were created for)
         - KDC will send a TGS (encrypted with the Service Owner Hash, the Service Owner is the user or machine account that the service runs under) along with a Service Session Key
     3) The TGS can then be sent to the desired service to authenticate and establish a connection. The service will use its configured account's password hash to decrypt the TGS and validate the Service Session Key.
- NetNTLM
    - `Legacy authentication protocol kept for compatibility purposes.`
    1. The client sends an authentication request to the server they want to access.
    2. The server generates a random number and sends it as a challenge to the client.
    3. The client combines their NTLM password hash with the challenge (and other known data) to generate a response to the challenge and sends it back to the server for verification.
    4. The server forwards the challenge and the response to the Domain Controller for verification.
    5. The domain controller uses the challenge to recalculate the response and compares it to the original response sent by the client. If they both match, the client is authenticated; otherwise, access is denied. The authentication result is sent back to the server.
    6. The server forwards the authentication result to the client.

While NetNTLM should be considered obsolete, most networks will have both protocols enabled. Let's take a deeper look at how each of these protocols works.