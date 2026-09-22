package com.knov.wrapper

import android.Manifest
import android.content.ActivityNotFoundException
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.ServiceConnection
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.os.IBinder
import android.provider.Settings
import android.view.Gravity
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat

class MainActivity : AppCompatActivity() {

    private companion object {
        const val PREF_ASKED_STORAGE_ACCESS = "asked_storage_access"
    }

    private lateinit var webView: WebView
    private var serverService: ServerService? = null
    private var serverStarted = false
    // set right before sending the user to the system "all files access" settings screen,
    // so onResume knows to pick the flow back up when they return - that screen has no
    // meaningful activity result to register for instead
    private var awaitingStorageSettings = false

    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName?, binder: IBinder?) {
            val service = (binder as ServerService.LocalBinder).getService()
            serverService = service
            service.addListener(::onStatus)
        }

        override fun onServiceDisconnected(name: ComponentName?) {
            serverService = null
        }
    }

    private val envEditorLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (result.resultCode == RESULT_OK) {
            showMessage(getString(R.string.starting_server))
        }
    }

    private val notificationPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val alreadyAsked = getPreferences(Context.MODE_PRIVATE).getBoolean(PREF_ASKED_STORAGE_ACCESS, false)
        val canAskStorageAccess = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && !Environment.isExternalStorageManager()
        if (canAskStorageAccess && !alreadyAsked) {
            showStoragePrompt()
        } else {
            setUpAppUi()
        }
    }

    override fun onResume() {
        super.onResume()
        // there's no meaningful activity result to register for from the system "all files
        // access" screen - onResume is what fires when the user comes back from it either way
        if (awaitingStorageSettings) {
            awaitingStorageSettings = false
            setUpAppUi()
        }
    }

    // set only once the user has actually chosen (grant or skip), not just on showing the
    // prompt - so a process death or back-press mid-prompt doesn't burn the one-time ask
    private fun markStorageAccessAsked() {
        getPreferences(Context.MODE_PRIVATE).edit().putBoolean(PREF_ASKED_STORAGE_ACCESS, true).apply()
    }

    // shown once, only if MANAGE_EXTERNAL_STORAGE hasn't been granted yet (see onCreate) -
    // lets the user opt into shared storage before the server starts, since ServerService
    // picks its data root at process-start time (see dataRootDir()), not something that can
    // be switched later without a restart
    private fun showStoragePrompt() {
        val message = TextView(this).apply {
            text = getString(R.string.storage_access_explanation)
            setPadding(48, 48, 48, 48)
        }
        val grantButton = Button(this).apply {
            text = getString(R.string.grant_storage_access)
            setOnClickListener {
                markStorageAccessAsked()
                awaitingStorageSettings = true
                startActivity(Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, Uri.parse("package:$packageName")))
            }
        }
        val skipButton = Button(this).apply {
            text = getString(R.string.skip_storage_access)
            setOnClickListener {
                markStorageAccessAsked()
                setUpAppUi()
            }
        }
        setContentView(
            LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER
                addView(message)
                addView(grantButton)
                addView(skipButton)
            },
        )
    }

    private fun setUpAppUi() {
        if (serverStarted) return

        webView = WebView(this)
        webView.settings.javaScriptEnabled = true
        webView.settings.domStorageEnabled = true
        // rendered file content can contain links; keep navigation confined to the
        // local server and hand everything else off to the system browser instead
        // of letting an untrusted page take over the JS-enabled WebView
        webView.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val uri = request.url
                val serverPort = serverService?.serverUrl()?.let { Uri.parse(it).port }
                if (uri.host == "127.0.0.1" && uri.port == serverPort) return false
                try {
                    startActivity(Intent(Intent.ACTION_VIEW, uri))
                } catch (e: ActivityNotFoundException) {
                    // link in file content with no app to handle it - nothing to do
                }
                return true
            }
        }

        val envButton = Button(this).apply {
            text = getString(R.string.edit_env)
            setOnClickListener { envEditorLauncher.launch(Intent(this@MainActivity, EnvEditorActivity::class.java)) }
        }

        setContentView(
            FrameLayout(this).apply {
                addView(webView, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
                addView(envButton, FrameLayout.LayoutParams(FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT, Gravity.TOP or Gravity.END))
            },
        )
        showMessage(getString(R.string.starting_server))

        // without this the persistent notification (and its Stop action) never
        // appears on API 33+, silently - the foreground service still runs fine
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }

        serverStarted = true
        val serviceIntent = Intent(this, ServerService::class.java)
        startForegroundService(serviceIntent)
        bindService(serviceIntent, connection, Context.BIND_AUTO_CREATE)
    }

    override fun onDestroy() {
        // bindService is only ever called from setUpAppUi(), so guard against unbinding a
        // connection that was never registered - reachable if the activity is destroyed while
        // still on the storage-access prompt screen (setUpAppUi hasn't run yet)
        if (serverStarted) {
            serverService?.removeListener()
            unbindService(connection)
        }
        super.onDestroy()
    }

    // fires on every status transition from ServerService, including ones after the
    // first successful load - e.g. the backend process crashing and restarting on its
    // own, not just the initial startup or an EnvEditorActivity-triggered restart -
    // so a dead server doesn't leave a stale, unresponsive page on screen unexplained
    private fun onStatus(status: ServerService.Status) {
        when (status) {
            ServerService.Status.READY -> {
                serverService?.serverUrl()?.let { webView.loadUrl(it) }
            }
            ServerService.Status.ERROR -> {
                showMessage(getString(R.string.server_start_failed))
            }
            ServerService.Status.STARTING -> {
                showMessage(getString(R.string.starting_server))
            }
        }
    }

    private fun showMessage(text: String) {
        val escaped = android.text.TextUtils.htmlEncode(text)
        webView.loadData(
            "<html><body style='font-family:sans-serif;text-align:center;padding-top:40%'>$escaped</body></html>",
            "text/html",
            "utf-8",
        )
    }
}
