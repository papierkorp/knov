# KNOV - Knowledge Management System

KNOV (knowledge vault) is a flexible knowledge management system.

# Core Features

- **Offline Available for all OS**
  - One Executable/Binary which can be carried on a USB Stick
  - Docker Version available
- **flexible Theme System**: 
  - Static assets and builtin theme assets are embedded in the binary for portable deployment.
  - builtin theme is unpacked on startup into the `KNOV_THEMES_PATH`
  - theme overwrite of specific templates
- **Search**
  - Strong Filter to display certain Files based on metadata
  - Multiple possible search backends (memory, grep, SQLite)
- **Strong Defaults**
  - allows for extensive customization but not neccessary

![image](filter_example.png)

- **Flat Files + GIT Integration**: 
  - Version control for your knowledge base with GIT
  - All your data in a git repo available as files and also accessible via text-editor/ide
  - different methods to add files
    - via app with a button
    - via file browser
    - via git push
  - quickly look through the history with all the changes the file had to endure
- **Dashboard System**: 
  - Customizable dashboards with a lot of widgets to display your data like you want
- **Multi-language Support**:
  - currently English and German translations
- **Organization with manually created Metadata**:
  - **Tags**: fully customizable tags for manual organization
  - **Parent**: set a file as a parent via selection to create a connection
- **Organization with automatically created Metadata**:
  - **collection**: organizational field to group related files - defaults to the first folder in filepath or "default" - can be changed manually
  - **connections**
    - if a file has an parent - the sytem automatically creates a connection system with a `Ancestor`, `Parents` and `Childs`
    - use markdown links in the content and you automatically get `links to - inbound links` and `links from - outbound links`
    - get related files per sqlite
  - **folders**: use a default Folder structure to get the filepath as well as see all folders clickable in the metadata


![image](metadata_example.png)

- **Add References without clustering the content**
  - Edit - References allows you to add URL References + Description to the metadata of a certain file

# Configuration

Configuration of the APP via ENV Variables.
KNOV can be deployed as a single binary with configurable paths:

- `KNOV_DATA_PATH`: Where your content files are stored
- `KNOV_THEMES_PATH`: Where theme .so files are located
- `KNOV_CONFIG_PATH`: Where configuration and user settings are stored
- `KNOV_SERVER_PORT`: HTTP server port

Take a look at the **.env.example** file for all possible configurations 

# Settings

Settings are stored as .json files in the `KNOV_CONFIG_PATH` for each binary individually

# Architecture

- **Backend**: Go with Chi router
- **Frontend**: HTMX + Go HTML templates
- **Storage**: local first storage with options for json files, sqlite or yaml headers in the documents directly
