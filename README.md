# temp

help me to implement the following:

- change the defaulttheme so it can be compiled to a .so file
- a function which searches and compiles all themes in the theme folder with: cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, ".")
- in Initialize fill the themes ThemeManager struct with the pluginnames
- a function which loads a specific theme .so file (the name of the theme is passed)
