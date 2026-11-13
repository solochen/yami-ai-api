UPDATE alert_channels SET notify_events = replace(notify_events, ',manual_migrate', '') WHERE POSITION('manual_migrate' IN notify_events) > 0;
