module Main exposing (main, view, update)

import Browser
import Html exposing (Html, button, div, text)
import Html.Events exposing (onClick)

type Msg
    = Increment
    | Decrement

type alias Model =
    { count : Int
    , label : String
    }

init : Model
init =
    { count = 0
    , label = "Counter"
    }

update : Msg -> Model -> Model
update msg model =
    case msg of
        Increment ->
            { model | count = model.count + 1 }

        Decrement ->
            { model | count = model.count - 1 }

view : Model -> Html Msg
view model =
    div []
        [ text (model.label ++ ": ")
        , button [ onClick Increment ] [ text "+" ]
        , text (String.fromInt model.count)
        , button [ onClick Decrement ] [ text "-" ]
        ]

main : Program () Model Msg
main =
    Browser.sandbox { init = init, view = view, update = update }
