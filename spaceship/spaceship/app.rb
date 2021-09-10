require 'spaceship'
require_relative 'portal/app_client'

def get_app(bundle_id)
  app = nil
  run_or_raise_preferred_error_message do
    app = Spaceship::Portal.app.find(bundle_id)
  end

  {
    id: app.app_id,
    bundleID: app.bundle_id,
    entitlements: app.details.features
  }
end

def create_app(bundle_id)
  app = nil
  run_or_raise_preferred_error_message do
    app = Spaceship::Portal.app.create!(bundle_id: bundle_id, name: name)
  end

  raise "failed to create app with bundle id: #{bundle_id}" unless app

  {
    id: app.app_id,
    bundleID: app.bundle_id,
  }
end

def check_bundleid(bundle_id, entitlements)
  app = nil
  run_or_raise_preferred_error_message do
    app = Spaceship::Portal.app.find(bundle_id)
  end

  Portal::AppClient.all_services_enabled?(app, entitlements)
end

def sync_bundleid(bundle_id, entitlements)
  app = nil
  run_or_raise_preferred_error_message do
    app = Spaceship::Portal.app.find(bundle_id)
  end

  Portal::AppClient.sync_app_services(app, entitlements)
end
