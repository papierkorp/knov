package com.knov.wrapper

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.Binder
import android.os.Build
import android.os.Environment
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import androidx.core.app.NotificationCompat
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import kotlin.math.min

// Runs the knov go binary as a plain child process and keeps it alive via a
// foreground service, so android doesn't kill it in the background. No
// JNI/gomobile bridge - the binary is unmodified, just spawned.
//
// The binary ships as a "native library" (jniLibs/<abi>/libknovserver.so)
// instead of an asset: since API 29, android's W^X policy blocks executing
// any file the app copies into its own data dir at runtime, even after
// chmod +x. Files under nativeLibraryDir are extracted executable at
// install time and are exempt from that restriction.
//
// Readiness/crash tracking lives here (not in MainActivity) since it's the
// service's own concern - MainActivity just binds and observes Status
// instead of running its own detached polling thread.
class ServerService : Service() {

    enum class Status { STARTING, READY, ERROR }

    companion object {
        // fallback when .env has no KNOV_SERVER_PORT line - matches configmanager/envdefs.go's default
        private const val DEFAULT_PORT = "1324"
        const val ACTION_STOP = "com.knov.wrapper.STOP"
        const val ACTION_RESTART = "com.knov.wrapper.RESTART"
        private const val TAG = "knov-server"
        private const val CHANNEL_ID = "knov-server"
        private const val NOTIFICATION_ID = 1
        private const val BINARY_NAME = "libknovserver.so"
        private const val POLL_INTERVAL_MS = 300L
        // once ready, crashes are already caught instantly via waitFor()/onProcessExit - the
        // poll only exists to notice the HTTP server coming up quickly during startup, so
        // there's nothing left to discover fast. Back off steeply instead of polling forever.
        private const val IDLE_POLL_INTERVAL_MS = 10_000L
        private const val BASE_BACKOFF_MS = 1000L
        private const val MAX_BACKOFF_MS = 30_000L
        private const val ERROR_THRESHOLD = 4
    }

    inner class LocalBinder : Binder() {
        fun getService(): ServerService = this@ServerService
    }

    private val mainHandler = Handler(Looper.getMainLooper())

    private val processLock = Any()
    private var process: Process? = null
    @Volatile private var stopping = false
    @Volatile private var restarting = false
    @Volatile private var ready = false
    @Volatile private var consecutiveFailures = 0
    private var listener: ((Status) -> Unit)? = null

    override fun onCreate() {
        super.onCreate()
        startForeground(NOTIFICATION_ID, buildNotification())
        synchronized(processLock) { process = startServerProcess() }
        startReadinessPolling()
    }

    override fun onBind(intent: Intent?): IBinder = LocalBinder()

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopSelf()
                return START_NOT_STICKY
            }
            ACTION_RESTART -> {
                ready = false
                notifyListener()
                // restarting is set/read under processLock alongside process itself, so a
                // restart requested while onProcessExit is mid-backoff (crash loop) can't
                // leak onto a later, unrelated crash - see onProcessExit
                synchronized(processLock) {
                    restarting = true
                    process?.destroy()
                }
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopping = true
        listener = null
        synchronized(processLock) {
            process?.destroy()
            process = null
        }
        super.onDestroy()
    }

    fun addListener(l: (Status) -> Unit) {
        listener = l
        l(currentStatus())
    }

    fun removeListener() {
        listener = null
    }

    private fun currentStatus(): Status = when {
        ready -> Status.READY
        consecutiveFailures >= ERROR_THRESHOLD -> Status.ERROR
        else -> Status.STARTING
    }

    private fun notifyListener() {
        val status = currentStatus()
        mainHandler.post { listener?.invoke(status) }
    }

    private fun onProcessExit() {
        if (stopping) return
        // a user-triggered restart (EnvEditorActivity) destroys the process deliberately -
        // don't count it as a crash or apply crash backoff to it. checked again after the
        // backoff sleep below too, in case a restart was requested mid crash-loop - otherwise
        // it'd sit unconsumed and get wrongly attributed to whatever unrelated crash exits next
        var wasRestarting: Boolean
        synchronized(processLock) {
            wasRestarting = restarting
            if (wasRestarting) {
                restarting = false
                consecutiveFailures = 0
            }
        }
        if (wasRestarting) {
            Log.i(TAG, "server process restarted")
            publishProcess(startServerProcess())
            return
        }
        consecutiveFailures++
        val backoff = min(BASE_BACKOFF_MS * (1L shl (consecutiveFailures - 1)), MAX_BACKOFF_MS)
        Log.w(TAG, "server process died, restarting in ${backoff}ms (failure #$consecutiveFailures)")
        notifyListener()
        Thread.sleep(backoff)
        // stopSelf() may have run while this thread was sleeping - without this check we'd
        // spawn a new, unsupervised process right after the user asked the service to stop
        if (stopping) return
        synchronized(processLock) {
            if (restarting) {
                restarting = false
                consecutiveFailures = 0
                Log.i(TAG, "server process restarted")
            }
        }
        publishProcess(startServerProcess())
    }

    // startServerProcess() runs unlocked (see onProcessExit), so by the time it returns, the
    // service may have been torn down or another restart may have been requested - re-check
    // under the lock and kill the just-spawned process instead of publishing it in that case
    private fun publishProcess(proc: Process?) {
        synchronized(processLock) {
            if (stopping) {
                proc?.destroy()
            } else {
                if (restarting) proc?.destroy()
                process = proc
            }
        }
    }

    private fun startReadinessPolling() {
        Thread {
            while (!stopping) {
                val up = isServerUp()
                if (up != ready) {
                    ready = up
                    if (up) consecutiveFailures = 0
                    notifyListener()
                }
                Thread.sleep(if (ready) IDLE_POLL_INTERVAL_MS else POLL_INTERVAL_MS)
            }
        }.start()
    }

    // .env's KNOV_SERVER_PORT (if set) wins over whatever we pass the child process - the go
    // binary applies .env onto its own env unconditionally (loadEnvFile in configmanager/config.go),
    // so a user editing .env via EnvEditorActivity can move the real port out from under us.
    // Read fresh each time rather than caching, so a port change takes effect after a restart.
    private fun currentPort(): String {
        val envFile = File(filesDir, ".env")
        if (!envFile.exists()) return DEFAULT_PORT
        for (line in envFile.readLines()) {
            val trimmed = line.trim()
            if (trimmed.isEmpty() || trimmed.startsWith("#")) continue
            // mirrors loadEnvFile's own parsing (SplitN on "=", trim key/value separately)
            // so this agrees with the go side on lines it'd otherwise disagree with, e.g. "KEY = value"
            val parts = trimmed.split("=", limit = 2)
            if (parts.size == 2 && parts[0].trim() == "KNOV_SERVER_PORT") {
                val value = parts[1].trim()
                if (value.isNotEmpty()) return value
            }
        }
        return DEFAULT_PORT
    }

    fun serverUrl(): String = "http://127.0.0.1:${currentPort()}/"

    // MANAGE_EXTERNAL_STORAGE is requested once at first launch (see MainActivity) so this can
    // live in shared storage, reachable from a normal file manager/USB, instead of the app's
    // private sandbox that only adb can reach. Falls back to the sandbox if never granted -
    // isExternalStorageManager() itself doesn't exist before API 30 (minSdk here is 26), so it
    // can't even be called on older devices, not just "returns false" there
    private fun dataRootDir(): File =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && Environment.isExternalStorageManager())
            File(Environment.getExternalStorageDirectory(), "knov")
        else filesDir

    private fun isServerUp(): Boolean {
        return try {
            val conn = URL(serverUrl()).openConnection() as HttpURLConnection
            conn.connectTimeout = 500
            conn.requestMethod = "GET"
            val ok = conn.responseCode in 200..499
            conn.disconnect()
            ok
        } catch (e: Exception) {
            false
        }
    }

    private fun startServerProcess(): Process? {
        val binaryPath = File(applicationInfo.nativeLibraryDir, BINARY_NAME).absolutePath
        try {
            val dataRoot = dataRootDir()
            val env = mapOf(
                "KNOV_SERVER_HOST" to "127.0.0.1",
                "KNOV_SERVER_PORT" to currentPort(),
                "KNOV_DATA_PATH" to File(dataRoot, "data").absolutePath,
                "KNOV_STORAGE_PATH" to File(dataRoot, "storage").absolutePath,
                "KNOV_THEMES_PATH" to File(dataRoot, "themes").absolutePath,
                "KNOV_LOGS_PATH" to File(dataRoot, "logs").absolutePath,
                "KNOV_BACKUPS_PATH" to File(dataRoot, "backups").absolutePath,
            )

            val builder = ProcessBuilder(binaryPath)
                .directory(filesDir)
                .redirectErrorStream(true)
            builder.environment().putAll(env)
            val proc = builder.start()

            Thread {
                proc.inputStream.bufferedReader().forEachLine { Log.i(TAG, it) }
            }.start()
            Thread {
                proc.waitFor()
                onProcessExit()
            }.start()

            return proc
        } catch (e: Exception) {
            Log.e(TAG, "failed to start server", e)
            // no process means no waitFor() thread to drive the retry loop - route this
            // through the same onProcessExit() backoff/retry path a crash would take,
            // instead of getting stuck in STARTING forever
            Thread { onProcessExit() }.start()
            return null
        }
    }

    private fun buildNotification(): android.app.Notification {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(CHANNEL_ID, "Knov server", NotificationManager.IMPORTANCE_LOW)
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
        val stopIntent = Intent(this, ServerService::class.java).setAction(ACTION_STOP)
        val stopPendingIntent = PendingIntent.getService(
            this, 0, stopIntent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val openPendingIntent = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.server_notification_title))
            .setContentIntent(openPendingIntent)
            .setSmallIcon(android.R.drawable.ic_menu_info_details)
            .setOngoing(true)
            .addAction(0, getString(R.string.stop_server), stopPendingIntent)
            .build()
    }
}
