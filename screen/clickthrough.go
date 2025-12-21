package main

/*
#cgo pkg-config: gtk+-3.0 cairo gdk-3.0
#include <gtk/gtk.h>
#include <gdk/gdk.h>
#include <cairo.h>

void set_click_through(GtkWidget *widget) {
    // Get the GDK window
    GdkWindow *gdk_window = gtk_widget_get_window(widget);
    if (gdk_window == NULL) {
        g_warning("GDK window is NULL, cannot set click-through");
        return;
    }

    // Create empty region for click-through
    cairo_region_t *region = cairo_region_create();

    // Set input shape on both GTK widget and GDK window
    gtk_widget_input_shape_combine_region(widget, region);
    gdk_window_input_shape_combine_region(gdk_window, region, 0, 0);

    // Set cursor to default (invisible/none would be better but default works)
    GdkCursor *cursor = gdk_cursor_new_from_name(gdk_display_get_default(), "default");
    gdk_window_set_cursor(gdk_window, cursor);
    g_object_unref(cursor);

    cairo_region_destroy(region);
}
*/
import "C"
import "unsafe"

// SetClickThrough makes a widget click-through by setting an empty input region.
// All mouse events will pass through to windows below.
func SetClickThrough(widgetPtr uintptr) {
	C.set_click_through((*C.GtkWidget)(unsafe.Pointer(widgetPtr)))
}
