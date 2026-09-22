package com.knov.wrapper

import android.content.Intent
import android.graphics.Typeface
import android.os.Bundle
import android.text.InputType
import android.view.Gravity
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import androidx.appcompat.app.AppCompatActivity
import java.io.File

// lets you edit the .env file the go server reads on startup (KNOV_* deployment
// config) without adb/a pc - there's no other way to get a file into the app's
// private storage from just the phone itself.
class EnvEditorActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val envFile = File(filesDir, ".env")

        val editText = EditText(this).apply {
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE
            typeface = Typeface.MONOSPACE
            gravity = Gravity.TOP
            setText(if (envFile.exists()) envFile.readText() else "")
        }

        val saveButton = Button(this).apply {
            text = getString(R.string.save_and_restart)
            setOnClickListener {
                envFile.writeText(editText.text.toString())
                startService(Intent(this@EnvEditorActivity, ServerService::class.java).setAction(ServerService.ACTION_RESTART))
                setResult(RESULT_OK)
                finish()
            }
        }

        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            addView(editText, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f))
            addView(
                saveButton,
                LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT),
            )
        }
        setContentView(layout)
    }
}
